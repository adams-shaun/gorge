package searchprobe

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

type epochKey struct {
	Player  state.PlayerID
	Ordinal int
}

type epochPosition struct {
	Index int
	Name  string
	Ref   uint32
}

type nameCount struct {
	Name  string
	Count int
}

type landIsolationConstraint struct {
	Through    int
	Name       string
	PriorExits []nameCount
}

type epochConstraints struct {
	Positions                []epochPosition
	Deadlines                []deadlineConstraint
	LandIsolation            []landIsolationConstraint
	LandIsolationUnsupported int
	ArrangeWindows           int
	Unguided                 []string
}

type epochCursor struct {
	ordinal      int
	draws        int
	reliable     bool
	handReliable bool
	handExits    map[string]int
	order        []int // remaining library positions in the epoch's original shuffle
	arrange      *epochArrange
}

type epochArrange struct {
	frame int
	order []int
}

func (c epochCursor) original(index int) int {
	if c.order == nil {
		return c.draws + index
	}
	if index >= len(c.order) {
		return -1
	}
	return c.order[index]
}

func compileEpochs(h History) (map[epochKey]epochConstraints, error) {
	epochs := make(map[epochKey]epochConstraints)
	cursors := make(map[state.PlayerID]epochCursor)
	deadlineCounts := make(map[epochKey]map[string]int)
	names := make(map[uint32]string)
	owners := make(map[uint32]state.PlayerID)
	for frameIndex, frame := range h.Frames {
		introduced := make(map[uint32]Identity, len(frame.Identities))
		for _, identity := range frame.Identities {
			// Stack abilities have observer identities but no underlying Card.
			// A name is required only when an identity is used as a card fact.
			if identity.ID == 0 {
				return nil, fail("contradictory", "observed identity has zero reference")
			}
			if old := names[identity.ID]; old != "" && old != identity.Name {
				return nil, fail("contradictory", "observed identity %d changed from %s to %s", identity.ID, old, identity.Name)
			}
			names[identity.ID] = identity.Name
			owners[identity.ID] = identity.Owner
			introduced[identity.ID] = identity
		}
		landCandidates, unsupportedLandPlays := frameLandIsolation(frame, names, owners, h.Actor)
		for eventIndex, ev := range frame.Events {
			if player, ok := unsupportedLandPlays[eventIndex]; ok {
				cursor := cursors[player]
				if cursor.ordinal > 0 {
					key := epochKey{Player: player, Ordinal: cursor.ordinal - 1}
					ep := epochs[key]
					ep.LandIsolationUnsupported++
					epochs[key] = ep
				}
			}
			cursor := cursors[ev.Player]
			switch ev.Kind {
			case events.Shuffle:
				cursor.ordinal++
				cursor.draws = 0
				cursor.reliable = true
				cursor.handReliable = true
				cursor.handExits = make(map[string]int)
				cursor.order, cursor.arrange = nil, nil
				cursors[ev.Player] = cursor
				ensureEpoch(epochs, epochKey{Player: ev.Player, Ordinal: cursor.ordinal - 1})
				continue
			case events.Draw:
				if ev.Player == h.Actor && ev.Obj != 0 && names[ev.Obj] == "" {
					return nil, fail("contradictory", "draw identity %d has no card name", ev.Obj)
				}
				if cursor.ordinal > 0 && cursor.reliable && ev.Player == h.Actor && ev.Obj != 0 {
					name := names[ev.Obj]
					key := epochKey{Player: ev.Player, Ordinal: cursor.ordinal - 1}
					ep := epochs[key]
					position := epochPosition{Index: cursor.original(0), Name: name}
					if position.Index < 0 {
						return nil, fail("contradictory", "draw follows an empty observed library")
					}
					if _, firstSeen := introduced[ev.Obj]; !firstSeen {
						position.Ref = ev.Obj
					}
					if err := addEpochPosition(&ep, position); err != nil {
						return nil, err
					}
					epochs[key] = ep
				}
				cursor.draws++
				cursor.arrange = nil
				if len(cursor.order) > 0 {
					cursor.order = cursor.order[1:]
				}
				cursors[ev.Player] = cursor
			}

			if ev.Kind == events.MoveZone && ev.To == state.ZHand && ev.From != state.ZHand {
				invalidateHandReliability(cursors, owners, ev.Obj)
			}
			if ev.From == state.ZHand && ev.To != state.ZHand {
				owner, ownerKnown := owners[ev.Obj]
				name := names[ev.Obj]
				visibleNamed := ev.Obj != 0 && ownerKnown && name != "" && !ev.Secret && !ev.To.Hidden()
				if !visibleNamed {
					invalidateHandReliability(cursors, owners, ev.Obj)
				} else {
					ownerCursor := cursors[owner]
					if candidateOwner, ok := landCandidates[eventIndex]; ok && candidateOwner == owner && ownerCursor.ordinal > 0 {
						key := epochKey{Player: owner, Ordinal: ownerCursor.ordinal - 1}
						ep := epochs[key]
						if ownerCursor.reliable && ownerCursor.handReliable {
							ep.LandIsolation = append(ep.LandIsolation, landIsolationConstraint{
								Through:    ownerCursor.draws,
								Name:       name,
								PriorExits: sortedNameCounts(ownerCursor.handExits),
							})
						} else {
							ep.LandIsolationUnsupported++
						}
						epochs[key] = ep
					}
					if ownerCursor.ordinal > 0 && ownerCursor.handReliable {
						ownerCursor.handExits[name]++
						cursors[owner] = ownerCursor
					}
				}
			}

			if identity, ok := introduced[ev.Obj]; ok && identity.Owner != h.Actor && ev.From == state.ZHand && !ev.To.Hidden() {
				if identity.Name == "" {
					return nil, fail("contradictory", "public hand-exit identity %d has no card name", ev.Obj)
				}
				delete(introduced, ev.Obj)
				ownerCursor := cursors[identity.Owner]
				if ownerCursor.ordinal > 0 && ownerCursor.reliable {
					key := epochKey{Player: identity.Owner, Ordinal: cursors[identity.Owner].ordinal - 1}
					if deadlineCounts[key] == nil {
						deadlineCounts[key] = make(map[string]int)
					}
					deadlineCounts[key][identity.Name]++
					ep := epochs[key]
					ep.Deadlines = append(ep.Deadlines, deadlineConstraint{Through: cursors[identity.Owner].draws, Name: identity.Name, Count: deadlineCounts[key][identity.Name]})
					epochs[key] = ep
				} else if ownerCursor.ordinal > 0 {
					key := epochKey{Player: identity.Owner, Ordinal: cursors[identity.Owner].ordinal - 1}
					ep := epochs[key]
					addUnguided(&ep, "opponent_hand_exit")
					epochs[key] = ep
				}
			}

			if ev.Kind == events.LibraryOrder {
				if cursor.reliable && ev.Player == h.Actor && cursor.arrange != nil && cursor.arrange.frame == frameIndex {
					cursor.order = cursor.arrange.order
					cursor.arrange = nil
					cursors[ev.Player] = cursor
					continue
				}
				if cursor.ordinal > 0 {
					key := epochKey{Player: ev.Player, Ordinal: cursor.ordinal - 1}
					ep := epochs[key]
					addUnguided(&ep, "library_order")
					epochs[key] = ep
				}
				cursor.reliable = false
				cursor.arrange = nil
				cursors[ev.Player] = cursor
			}
			if ev.Kind == events.MoveZone && (ev.From == state.ZLibrary || ev.To == state.ZLibrary) {
				owner, known := owners[ev.Obj]
				// Player is the effect controller at some MoveZone sites. Without
				// an observed owner, invalidate every active cursor, not a guess.
				for player, affected := range cursors {
					if known && player != owner {
						continue
					}
					if affected.ordinal > 0 {
						key := epochKey{Player: player, Ordinal: affected.ordinal - 1}
						ep := epochs[key]
						addUnguided(&ep, "library_mutation")
						epochs[key] = ep
					}
					affected.reliable = false
					affected.arrange = nil
					cursors[player] = affected
				}
			}
		}
		if d := frame.Decision; d != nil && d.Player == h.Actor && d.Kind == decision.KArrange {
			cursor := cursors[d.Player]
			if cursor.ordinal <= 0 {
				return nil, fail("contradictory", "arrange decision precedes library shuffle")
			}
			key := epochKey{Player: d.Player, Ordinal: cursor.ordinal - 1}
			ep := epochs[key]
			for i, option := range d.Options {
				ref := option.Action.Obj
				name := names[ref]
				if ref == 0 || name == "" {
					return nil, fail("contradictory", "arrange option %d has no observed identity", i)
				}
				if cursor.reliable {
					if err := addEpochPosition(&ep, epochPosition{Index: cursor.original(i), Name: name, Ref: ref}); err != nil {
						return nil, err
					}
				}
			}
			if cursor.reliable {
				ep.ArrangeWindows++
				if answer, answered := h.Answers[frameIndex]; answered {
					if order, ok := arrangedEpochPositions(frame, cursor, answer); ok {
						cursor.arrange = &epochArrange{frame: frameIndex + 1, order: order}
						cursors[d.Player] = cursor
					}
				}
			} else {
				addUnguided(&ep, "arrange_window")
			}
			epochs[key] = ep
		}
	}
	return epochs, nil
}

func frameLandIsolation(frame Frame, names map[uint32]string, owners map[uint32]state.PlayerID, actor state.PlayerID) (map[int]state.PlayerID, map[int]state.PlayerID) {
	plays := make(map[state.PlayerID][]int)
	moves := make(map[state.PlayerID][]int)
	for i, ev := range frame.Events {
		if ev.Kind == events.LandPlayed && ev.Player != actor {
			plays[ev.Player] = append(plays[ev.Player], i)
		}
		if ev.Kind != events.MoveZone || ev.From != state.ZHand || ev.To != state.ZBattlefield || ev.Secret || ev.Obj == 0 || names[ev.Obj] == "" {
			continue
		}
		if owner, ok := owners[ev.Obj]; ok && owner != actor {
			moves[owner] = append(moves[owner], i)
		}
	}
	candidates := make(map[int]state.PlayerID)
	unsupported := make(map[int]state.PlayerID)
	for player, indices := range plays {
		if len(indices) == 1 && len(moves[player]) == 1 {
			candidates[moves[player][0]] = player
			continue
		}
		for _, index := range indices {
			unsupported[index] = player
		}
	}
	return candidates, unsupported
}

func invalidateHandReliability(cursors map[state.PlayerID]epochCursor, owners map[uint32]state.PlayerID, ref uint32) {
	if owner, ok := owners[ref]; ok {
		cursor := cursors[owner]
		cursor.handReliable = false
		cursors[owner] = cursor
		return
	}
	for player, cursor := range cursors {
		cursor.handReliable = false
		cursors[player] = cursor
	}
}

func sortedNameCounts(counts map[string]int) []nameCount {
	names := make([]string, 0, len(counts))
	for name, count := range counts {
		if count > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	out := make([]nameCount, 0, len(names))
	for _, name := range names {
		out = append(out, nameCount{Name: name, Count: counts[name]})
	}
	return out
}

// The owned board supplies only the public library size. Semantic actor
// answers identify option references; private/raw option indices never enter.
func arrangedEpochPositions(frame Frame, cursor epochCursor, answer []Action) ([]int, bool) {
	d := frame.Decision
	if len(answer) < d.Min || len(answer) > d.Max || len(d.Options) == 0 {
		return nil, false
	}
	var board struct {
		Players []struct {
			ID          state.PlayerID `json:"seat"`
			LibrarySize *int           `json:"library_size"`
		} `json:"players"`
	}
	if json.Unmarshal(frame.Board, &board) != nil {
		return nil, false
	}
	size := -1
	for _, player := range board.Players {
		if player.ID == d.Player && player.LibrarySize != nil {
			size = *player.LibrarySize
		}
	}
	if size < len(d.Options) || cursor.order != nil && len(cursor.order) != size {
		return nil, false
	}
	kind := d.Options[0].Action.Kind
	if kind != "" && kind != "top" && kind != "bottom" {
		return nil, false
	}
	for _, option := range d.Options {
		if option.Action.Kind != kind {
			return nil, false
		}
	}
	chosen := make([]bool, len(d.Options))
	order := make([]int, 0, size)
	for _, action := range answer {
		found := -1
		for i, option := range d.Options {
			if action == option.Action {
				if found >= 0 {
					return nil, false
				}
				found = i
			}
		}
		if found < 0 || chosen[found] {
			return nil, false
		}
		chosen[found] = true
		order = append(order, cursor.original(found))
	}
	var unchosen []int
	for i := range d.Options {
		if !chosen[i] {
			unchosen = append(unchosen, cursor.original(i))
		}
	}
	if kind != "bottom" {
		order = append(order, unchosen...)
	}
	for i := len(d.Options); i < size; i++ {
		order = append(order, cursor.original(i))
	}
	if kind == "bottom" {
		order = append(order, unchosen...)
	}
	return order, true
}

func ensureEpoch(epochs map[epochKey]epochConstraints, key epochKey) {
	if _, ok := epochs[key]; !ok {
		epochs[key] = epochConstraints{}
	}
}

func addEpochPosition(epoch *epochConstraints, position epochPosition) error {
	for _, existing := range epoch.Positions {
		if position.Ref != 0 && existing.Ref == position.Ref && existing.Index != position.Index {
			return fail("contradictory", "observed object %d occupies shuffle positions %d and %d", position.Ref, existing.Index, position.Index)
		}
		if existing.Index != position.Index {
			continue
		}
		if existing.Name != position.Name || existing.Ref != position.Ref {
			return fail("contradictory", "position %d requires both %s and %s", position.Index, existing.Name, position.Name)
		}
		return nil
	}
	epoch.Positions = append(epoch.Positions, position)
	return nil
}

func addUnguided(epoch *epochConstraints, reason string) {
	for _, existing := range epoch.Unguided {
		if existing == reason {
			return
		}
	}
	epoch.Unguided = append(epoch.Unguided, reason)
}

func (k epochKey) String() string {
	return fmt.Sprintf("player %d shuffle %d", k.Player, k.Ordinal)
}
