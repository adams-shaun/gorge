package effects

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// changeZoneAllPlayers resolves the player scope Forge's ChangeZoneAllEffect
// applies. A card that says "exile all cards from target player's graveyard"
// (Bojuka Bog, Tormod's Crypt, Nihil Spellbomb, ...) names ONE player and must
// move only that player's cards; without this scope the effect swept every
// player's zones. Forge's rule:
//
//	if ((!sa.usesTargeting() && !sa.hasParam("Defined")) || UseAllOriginZones$ True)
//	    -> every player
//	else
//	    -> the chosen target players when the ability uses targeting, else the
//	       Defined$ players (getTargetPlayers; targeting wins when both ride)
//
// Both scope forms resolve through the shared player-target vocabulary
// (`Defined` / `definedSpec`), so `ValidTgts$ Player`, `ValidTgts$ Opponent`,
// `Defined$ You`, `Defined$ TargetedController` and the rest all work without a
// second spelling table. On the non-targeting path, a selector we cannot
// resolve to a PLAYER (an unknown spelling, or an object-only one) keeps the
// pre-fix all-players sweep rather than silently moving nothing, and emits a
// Note saying so: the unscoped sweep is the previous behaviour, so an
// unmodelled card is never quietly inert, and a card we DO understand is
// correctly restricted. On the TARGETING path an empty chosen set is a legal
// answer ("any number", CR 601.2c, includes zero), so it scopes to no players
// and moves nothing -- Thraben Charm's "exile any number of target players'
// graveyards" cast with no targets must not sweep every graveyard. `Origin$`
// handling is not
// touched -- ParseZones already splits `Hand,Graveyard` into two zones and the
// Any/All wildcard is resolved by the caller before this runs.
func changeZoneAllPlayers(h Host, c *Ctx, sa *cards.SA, p *ChangeZoneAllParams) (players []state.PlayerID, scoped bool) {
	g := h.Game()
	if p.UseAllOriginZones {
		return g.AliveFrom(0), true
	}
	targeting := p.Targeting
	if !targeting && !p.DefinedPresent {
		return g.AliveFrom(0), false
	}
	var chosen []state.Target
	if targeting {
		// Forge's getTargetPlayers reads the SA's ANSWERED target players when
		// it uses targeting, never Defined$; the pre-ask answered set outranks
		// the resolution list exactly as Defined's own targeting branch does.
		if c.PickedTargets != nil {
			chosen = c.PickedTargets
		} else {
			chosen = c.Targets
		}
	} else {
		chosen = Defined(h, c, sa)
	}
	seen := make(map[state.PlayerID]bool, len(chosen))
	out := make([]state.PlayerID, 0, len(chosen))
	for _, t := range chosen {
		if !t.IsPlayer || seen[t.Player] {
			continue
		}
		seen[t.Player] = true
		out = append(out, t.Player)
	}
	if len(out) == 0 {
		if targeting {
			// CR 601.2c: "any number" includes zero. An empty chosen set on
			// the targeting path is a legal answer that names no players, not
			// an unresolvable scope -- the effect moves nothing.
			return nil, true
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "ChangeZoneAll could not resolve a player scope from " + p.DefinedText + "; sweeping all players"})
		return g.AliveFrom(0), false
	}
	return out, true
}

const ashiokPayLifeChangeType = "Card.NotDefinedReplacedSimultaneousETB"

// triggeredCardsChangeType is the ChangeType$ spelling of the
// trigger-Remembered referent effChangeZoneAll resolves from c.Remembered.
const triggeredCardsChangeType = "Card.TriggeredCards"

func changeZoneAllTypeLimitSpec(spec string) string {
	if spec == ashiokPayLifeChangeType {
		return "Card"
	}
	return spec
}

func changeZoneAllCap(h Host, c *Ctx, cza *ChangeZoneAllParams) int32 {
	cap := int32(-1)
	value := cza.ChangeNum
	if !cza.ChangeNumCapped {
		value = cza.TypeLimit
		if value.Text == "" {
			return cap
		}
	}
	cap = numText(h, c, value, 0)
	if cap < 0 {
		return 0
	}
	return cap
}

// changeZoneAllMatcher is effChangeZoneAll's one membership test: the
// trigger-captured set, the ForgetOtherRemembered$ snapshot, or the zone
// filter, in that order of precedence. The zone filter is zone-aware: a scan
// of any zone OTHER than the battlefield reads a leading Permanent base as
// Forge's "permanent card" (permanentCardSpec, the Dig/search rule), so a
// mixed Origin$ that includes the battlefield still takes graveyard cards as
// permanent cards (Once More with Feeling's "Exile all permanents and all
// cards from all graveyards") while the battlefield scan keeps the
// on-the-battlefield object reading. For a battlefield-excluding sweep every
// scan is non-battlefield, which is exactly the whole-origin rewrite the
// pre-per-zone matcher applied.
func changeZoneAllMatcher(g *state.Game, c *Ctx, spec string, triggeredSet map[state.ObjID]bool, preMatched *map[state.ObjID]bool) func(state.Zone, state.ObjID) bool {
	return func(zone state.Zone, id state.ObjID) bool {
		switch {
		case triggeredSet != nil:
			return triggeredSet[id]
		case preMatched != nil && *preMatched != nil:
			return (*preMatched)[id]
		}
		s := spec
		if zone != state.ZBattlefield {
			s = permanentCardSpec(s)
		}
		return MatchesSpecCtx(g, s, id, c.SpecContext(c.Controller))
	}
}

// changeZoneAllOwners lists the owner of every object the sweep would move,
// with the sweep's own scan order and shared-stack guard (the stack is
// visited once, under the first scoped player).
func changeZoneAllOwners(g *state.Game, from []state.Zone, players []state.PlayerID, matches func(state.Zone, state.ObjID) bool) []state.PlayerID {
	var owners []state.PlayerID
	for _, z := range from {
		for qi, p := range players {
			if z == state.ZStack && qi > 0 {
				continue
			}
			for _, id := range g.Zone(z, p) {
				if !matches(z, id) {
					continue
				}
				if o := g.Obj(id); o != nil {
					owners = append(owners, o.Owner)
				}
			}
		}
	}
	return owners
}

func effChangeZoneAll(h Host, c *Ctx, sa *cards.SA) {
	cza := ChangeZoneAllOf(sa)
	noteUnreadParams(h, c, "ChangeZoneAll", cza.Unread)
	if exileHostGoneFor(h, c, cza.Riders.Duration) {
		return
	}
	from := cza.Origin
	if !cza.OriginOK {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognised ChangeZoneAll Origin " + cza.OriginText})
		return
	}
	to := cza.Destination
	spec := changeZoneAllTypeLimitSpec(cza.ChangeType)
	// The leading-Permanent base rewrite now happens per scan zone, inside
	// changeZoneAllMatcher -- a mixed Origin$ that includes the battlefield
	// still reads the base as a permanent card in its non-battlefield scans.
	// ChangeType$ Card.TriggeredCards (Hedge Shredder's "Whenever one or more
	// land cards are put into your graveyard from your library, put them onto
	// the battlefield tapped", Toluz, Clever Conductor's exile sweep): the
	// trigger-Remembered referent names the cards the triggering zone change
	// captured -- a set the zone-filter grammar cannot bind (the same
	// unbindable shape clone.go's choice grammar records). It resolves from
	// the trigger's Remembered half instead of a zone scan, and the sweep's
	// Origin$ zones still bound it: a remembered object outside them is
	// simply never visited. The set is snapshotted before any move so the
	// sweep cannot match the moved cards it creates.
	triggeredSet := map[state.ObjID]bool(nil)
	if strings.EqualFold(spec, triggeredCardsChangeType) {
		triggeredSet = make(map[state.ObjID]bool)
		for _, t := range c.Remembered {
			if !t.IsPlayer && t.Obj != 0 {
				triggeredSet[t.Obj] = true
			}
		}
	}
	// ChangeNum$ caps the sweep (expert_level_safe's DBOpenSafe writes "All",
	// bone_dancer's DBChangeZone writes "1"): an omitted value or "All" moves
	// every matching card -- the behaviour the primitive always had -- while a
	// numeric cap (a literal, or an SVar/X reference through Num) moves at
	// most that many, in the sweep's own scan order (zone-major, seat-minor;
	// a RandomOrder$ sweep's shuffle picks WHICH candidates sit under the
	// cap, since the shuffle only sets the move order). A value Num cannot
	// resolve degrades to 0 by Num's own documented convention -- "the card
	// did nothing", the fail-closed direction.
	changeCap := changeZoneAllCap(h, c, cza)
	g := h.Game()
	// LibraryPosition$ (Terminus' "put all creatures on the bottom of their
	// owners' libraries") and Shuffle$ (Jace, the Mind Sculptor's [-12]
	// "shuffles their hand into their library", Gomazoa's "put on top ... then
	// those players shuffle") both act on the DESTINATION libraries, which are
	// each object's OWNER's library — a battlefield creature controlled by
	// another player (the Gomazoa / Vortex Elemental blocking shapes) still
	// returns to its owner's library, because the MoveZone keeps its owner.
	// The move loop therefore records every destination-library OWNER that had
	// a card moved (read off the object, not the source-zone player), in the
	// loop's own deterministic (zone-major, AliveFrom(0)-minor) order.
	position := cza.LibraryPosition
	// NoShuffle$ (the TwoPiles pile body's forced opt-out) wins over an
	// explicit Shuffle$ True, exactly as ChangeZone's objectPathShuffleOwed
	// resolves the pair; noShuffleBody only forces it on a body that named
	// neither flag, so the two never actually co-occur.
	shuffle := cza.ShuffleTrue && !cza.NoShuffle
	var placements []ownerMoved
	// AtEOT$'s affected set for ChangeZoneAll is the objects the sweep
	// actually moved, collected in move order.
	var moved []state.ObjID
	findOwnerMoved := func(owner state.PlayerID) *ownerMoved {
		for i := range placements {
			if placements[i].owner == owner {
				return &placements[i]
			}
		}
		placements = append(placements, ownerMoved{owner: owner})
		return &placements[len(placements)-1]
	}
	players, scoped := changeZoneAllPlayers(h, c, sa, cza)
	var preMatched map[state.ObjID]bool
	// The one membership test, defined before the ForgetOtherRemembered$
	// snapshot so the snapshot's own scan shares it: preMatched is captured
	// by POINTER (a nil value captured by copy would stay nil for the
	// closure however the caller assigns it later), so while the snapshot is
	// being built the matcher falls through to the zone filter, and every
	// later call reads the published snapshot.
	matches := changeZoneAllMatcher(g, c, spec, triggeredSet, &preMatched)
	// ForgetOtherRemembered$ True (The Mimeoplasm's MimeoExile, 11 corpus
	// ChangeZoneAll carriers): Forge forgets every previously remembered
	// object before this effect resolves, so a setup that remembered its own
	// candidates (the ChooseCard's RememberChosen$) plus stale memory from an
	// earlier resolution leaves exactly the moved set behind (RememberChanged$
	// re-remembers it). The ChangeType$ Card.IsRemembered selector reads the
	// very memory the clear drops, so the matched set is snapshotted BEFORE
	// the clear and the sweep below matches against the snapshot -- matching
	// after the clear would sweep nothing.
	if cza.Riders.ForgetOtherRemembered {
		// The snapshot is built into a local and only then published: the
		// matcher itself reads preMatched, so an empty map published before
		// the snapshot would shadow the zone filter with all-false.
		pre := make(map[state.ObjID]bool)
		for _, z := range from {
			for _, p := range players {
				for _, id := range g.Zone(z, p) {
					if matches(z, id) {
						pre[id] = true
					}
				}
			}
		}
		preMatched = pre
		c.Remembered = nil
		clearEventRemembered(h, c)
	}
	// RandomOrder$ True (task mordorparams1, Gríma, Saruman's Footman's
	// "Then that player puts the exiled cards that weren't cast this way on
	// the bottom of their library in a random order"): the destination
	// placement order is a real shuffle, not the engine's scan order. The
	// cards are COLLECTED first (the same zone-major/seat-minor scan, no
	// emission), Fisher-Yates'd per destination-library owner through the
	// seeded engine rng (the randomChoices/Host.Rand precedent — a replay
	// re-derives the identical order), and only then emitted, so the
	// MoveZone appends settle in the shuffled order. The shuffle only sets
	// the MOVE ORDER; the LibraryPosition$/Shuffle$ tail below still applies
	// on top of it (Triumph of Saint Katherine's `LibraryPosition$ 0` after
	// a RandomOrder$ sweep puts the shuffled pile on TOP, not the bottom).
	// The Dig/RestRandomOrder$/RevealRandomOrder$ variants are their own rows
	// and are not touched here.
	randomOrder := cza.RandomOrder
	// The Attacking$ rider's CR 508.4 chooser is the entering permanent's
	// controller. A single-owner sweep asks that owner; a mixed-owner sweep
	// falls back to the resolving controller (uniformEnteringController).
	var sweepOwners []state.PlayerID
	if strings.EqualFold(cza.Riders.Attacking, "True") && to == state.ZBattlefield {
		sweepOwners = changeZoneAllOwners(g, from, players, matches)
	}
	rider := classifyAttackingEntryText(h, c, cza.Riders.Attacking, to,
		enteringControllerForAttack(h, c, cza.Riders.Attacking, to, sweepOwners, cza.Riders.GainControl))
	// A sweep off the battlefield is one simultaneous departure (CR 603.10a):
	// every member's leaves-the-battlefield triggers look back at the same
	// pre-sweep board. The id list is empty -- the sweep emits as it scans --
	// so only the shared board is parked; the per-object lifelink capture
	// keeps its live read.
	if to != state.ZBattlefield && slices.Contains(from, state.ZBattlefield) {
		h.BatchDepartures(nil)
		defer h.EndBatchDepartures()
	}
	// A sweep onto the battlefield is one entering event (CR 603.6a): the zone batch holds each entry's trigger check until the last card has entered, so the entrants see each other, and a "one or more" line triggers once (CR 603.2c).
	if to == state.ZBattlefield {
		h.BeginZoneBatch()
		defer h.EndZoneBatch()
	}
	emitMove := func(id state.ObjID, z state.Zone, p state.PlayerID) {
		// CR 111.8: a token that has left the battlefield can't move to
		// another zone or come back onto the battlefield. Skipping it here
		// (no MoveZone, no remember bookkeeping) leaves it where it is until
		// ceaseDeadTokens retires it, the same guard ChangeZone's mover
		// applies through changeZoneMoveSkip.
		if o := g.Obj(id); o != nil && tokenCannotChangeZone(o) {
			return
		}
		// A CantExile restriction withholds the object from a battlefield exile
		// before the MoveZone (and the moved bookkeeping) is produced -- the
		// ChangeZoneAll half of the same guard effChangeZone's object loop and
		// settleChangeZoneMoveAs carry.
		if to == state.ZExile && h.ExileBlocked(id, false) {
			return
		}
		// Capture before MoveZone folds: battlefield departure resets control,
		// clears counters and removes battlefield-derived characteristics.
		if cza.RememberLKI {
			if o := g.Obj(id); o != nil {
				snapshot := o.CloneDeep()
				c.Snap.ChangeZone = append(c.Snap.ChangeZone, state.LKIObject{
					Obj: id, Controller: o.Controller, Owner: o.Owner, Snapshot: snapshot,
				})
			}
		}
		ev := moveZoneEvent(c, id, z, to)
		applyMoveFaceDown(h, c, &cza.Riders.FaceDownRiders, &ev, to)
		// WithCountersType$ folds into the entry event so the permanent
		// ENTERS with its counters (CR 122.6) and its enters triggers see
		// them -- Dig's WithCounters$ fold, ChangeZone's entry-counter path.
		if cza.WithCountersType != "" {
			entryWithCounters(&ev, to, cza.WithCountersType, numText(h, c, cza.WithCountersAmount, 1))
		}
		emitEntryMove(h, c, cza.Riders.GainControl, to, ev)
		moved = append(moved, id)
		exiledWithAssociation(h, c, id, to)
		// Tapped$ True (Splendid Reclamation's "Return all land cards
		// ... tapped"): a battlefield entry is followed by the same
		// "entered tapped" Tap event every other Tapped$ zone-change
		// path emits -- an entry state, not the CR 701.21a event of
		// becoming tapped.
		if to == state.ZBattlefield && cza.Tapped {
			h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: p, Text: "entered tapped"})
		}
		rider.apply(h, c, id, p, to)
		if to == state.ZExile {
			recordExileReturnFor(h, c, sa, cza.Riders.Duration, id, z, to)
		}
		// GainControl$ hands the moved object to the named player
		// (Karn Liberated's ReturnFromExile, Cold Storage, Ghost
		// Vacuum). Only a battlefield entry can carry a control
		// change (CR 701.22a controls permanents), the same rule the
		// ChangeZone path applies; the shared resolver is loud rather
		// than silent on an unresolvable selector.
		if to == state.ZBattlefield {
			// StaticEffect$ (ChangeZoneAll's carriers -- Ghost Vacuum, Grimoire
			// of the Dead, Storm of Souls, Shilgengar): the same per-card rider
			// registration every other ChangeZone mover applies.
			applyStaticEffect(h, c, sa, to, []state.ObjID{id})
		}
		if to == state.ZLibrary {
			owner := p
			if o := g.Obj(id); o != nil {
				owner = o.Owner
			}
			findOwnerMoved(owner).ids = append(findOwnerMoved(owner).ids, id)
		}
		// RememberChanged$ True re-remembers the moved cards in both halves
		// (the ctx list the chain's later sub-abilities read and the source's
		// event-backed persistent list a later resolution's IsRemembered /
		// Remembered$ head reads -- The Mimeoplasm's MimeoChooseCopy, Gift of
		// Immortality's return trigger). Previously this recorded the ctx
		// entries alone and only for the ExiledWithSource provenance shape
		// (Valakut Exploration); the persistent half is what the Mimeoplasm
		// chain's IsRemembered/Remembered$CardPower reads need.
		if cza.RememberLKI && !cza.RememberChanged {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
		if cza.RememberChanged {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
			eventRemember(h, c, id)
		}
		// ForgetChanged$ True runs after the remember handling (Forge
		// ChangeZoneAllEffect order), so remember + forget leaves the card out.
		eventForgetChanged(h, c, sa, id)
	}
	if randomOrder {
		type pendingMove struct {
			id state.ObjID
			z  state.Zone
			p  state.PlayerID
		}
		var owners []state.PlayerID
		byOwner := make(map[state.PlayerID][]pendingMove)
		for _, z := range from {
			for qi, p := range players {
				// The shared stack (engine/state/game.go Zone) is snapshotted once,
				// under the first player in the resolved scope. Without this an
				// N-player sweep enqueues the same stack object N times and
				// emits N MoveZones for one card. Other origins stay per-player.
				if z == state.ZStack && qi > 0 {
					continue
				}
				// Snapshot the zone exactly like the emit loop does.
				ids := append([]state.ObjID(nil), g.Zone(z, p)...)
				for _, id := range ids {
					if !matches(z, id) {
						continue
					}
					owner := p
					if o := g.Obj(id); o != nil {
						owner = o.Owner
					}
					if _, seen := byOwner[owner]; !seen {
						owners = append(owners, owner)
					}
					byOwner[owner] = append(byOwner[owner], pendingMove{id: id, z: z, p: p})
				}
			}
		}
		for _, owner := range owners {
			list := byOwner[owner]
			for i := len(list) - 1; i > 0; i-- {
				j := h.Rand(i + 1)
				list[i], list[j] = list[j], list[i]
			}
			byOwner[owner] = list
		}
	emitLoop:
		for _, owner := range owners {
			for _, pm := range byOwner[owner] {
				if changeCap >= 0 && int32(len(moved)) >= changeCap {
					break emitLoop
				}
				emitMove(pm.id, pm.z, pm.p)
			}
		}
	} else {
	sweep:
		for _, z := range from {
			for qi, p := range players {
				// Same shared-stack guard as the RandomOrder$ branch: one
				// snapshot of the stack, taken under the first scoped player.
				if z == state.ZStack && qi > 0 {
					continue
				}
				// Snapshot the zone: emitting move events mutates it underneath us.
				ids := append([]state.ObjID(nil), g.Zone(z, p)...)
				for _, id := range ids {
					if changeCap >= 0 && int32(len(moved)) >= changeCap {
						break sweep
					}
					if matches(z, id) {
						emitMove(id, z, p)
					}
				}
			}
		}
	}
	// The post-placement tail runs for BOTH branches: the shuffled order is
	// only the ORDER the moves settle in, so `LibraryPosition$ 0` (Triumph of
	// Saint Katherine's "shuffle that pile and put it back on TOP of your
	// library") and `Shuffle$` must still apply after a RandomOrder$ sweep.
	if to == state.ZLibrary && len(placements) > 0 {
		// LibraryPosition$: MoveZone already appends at the bottom of the
		// destination library in settle order, so "-1" (Terminus) is exactly the
		// move order and needs no extra event; "0" pins the moved cards on TOP
		// via the one Secret LibraryOrder placement every library placement
		// shares. The ABSENT spelling is also TOP (golgari_thug2):
		// ChangeZoneAllEffect computes libPos = 0 when LibraryPosition$ is
		// absent, the same default the hand path and the object-target path
		// apply. Any other value is loud rather than silently inert.
		switch effChangeZoneAllCodes.Code(string(position)) {
		case effChangeZoneAllBottom:
		case effChangeZoneAllTop:
			for _, pm := range placements {
				libraryOrderPlacement(h, pm.owner, pm.ids, false)
			}
		default:
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "LibraryPosition$ " + position + " is not implemented; the cards sit at the BOTTOM of their owners' libraries (the MoveZone append)"})
		}
	}
	if to == state.ZLibrary && shuffle {
		changeZoneAllShuffleLibraries(h, placements, players, scoped)
	}
	scheduleAtEOT(h, c, sa, moved)
}

// ownerMoved is one destination library's share of a ChangeZoneAll sweep:
// the receiving player and the objects that landed there, in settle order.
type ownerMoved struct {
	owner state.PlayerID
	ids   []state.ObjID
}

// changeZoneAllShuffleLibraries is the ChangeZoneAll library-shuffle tail.
// Shuffle$ True shuffles each destination library that received a card,
// AFTER the placement (Gomazoa's "put on top ..., then those players
// shuffle" order), through the same Secret events.Shuffle every other
// library shuffle emits, and then -- CR 701.24d, "that library is
// shuffled even if there are no objects in that set" -- every library of
// an EXPLICIT player scope (Defined$ / ValidTgts$ / UseAllOriginZones$:
// Mnemonic Nexus's "each player", Timetwister, Blessed Respite's target
// player) that received nothing, in scope order. The unscoped
// all-players default (the "your graveyard" Card.YouOwn shapes) and the
// unresolved-scope fallback shuffle only the receiving libraries.
// NoShuffle$ True (the forced TwoPiles opt-out) is the caller's gate.
func changeZoneAllShuffleLibraries(h Host, placements []ownerMoved, players []state.PlayerID, scoped bool) {
	owed := make([]state.PlayerID, 0, len(placements)+len(players))
	for _, pm := range placements {
		owed = append(owed, pm.owner)
	}
	if scoped {
		for _, p := range players {
			if !slices.Contains(owed, p) {
				owed = append(owed, p)
			}
		}
	}
	for _, p := range owed {
		shuffleLibraryOrder(h, p)
	}
}

type effChangeZoneAllCode uint16

const (
	effChangeZoneAllBottom effChangeZoneAllCode = iota + 1
	effChangeZoneAllTop
)

var effChangeZoneAllCodes = state.NewStrCodes(
	state.StrEntry[effChangeZoneAllCode]{Key: "-1", Val: effChangeZoneAllBottom},
	state.StrEntry[effChangeZoneAllCode]{Key: "", Val: effChangeZoneAllTop},
	state.StrEntry[effChangeZoneAllCode]{Key: "0", Val: effChangeZoneAllTop},
)
