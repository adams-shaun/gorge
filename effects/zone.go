package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ChangeZone", effChangeZone)
	Register("ChangeZoneAll", effChangeZoneAll)
	Register("Destroy", effDestroy)
	Register("DestroyAll", effDestroyAll)
	Register("Sacrifice", effSacrifice)
}

// ParseZone maps a Forge zone name to a state.Zone. Unknown names resolve to
// the graveyard, which is where the overwhelming majority of movement goes
// and is a safe default for an unmodelled destination.
func ParseZone(s string) state.Zone {
	z, ok := parseZone(s)
	if !ok {
		return state.ZGraveyard
	}
	return z
}

func parseZone(s string) (state.Zone, bool) {
	switch strings.TrimSpace(s) {
	case "Hand":
		return state.ZHand, true
	case "Battlefield":
		return state.ZBattlefield, true
	case "Library":
		return state.ZLibrary, true
	case "Graveyard":
		return state.ZGraveyard, true
	case "Exile":
		return state.ZExile, true
	case "Stack":
		return state.ZStack, true
	case "Command":
		return state.ZCommand, true
	case "Ceased":
		return state.ZCeased, true
	}
	return 0, false
}

// ParseZones parses an Origin$ zone set. Any and All are wildcards. A false
// result means at least one token was unknown; callers must not treat it as a
// graveyard origin.
func ParseZones(s string) (zones []state.Zone, all, ok bool) {
	ok = true
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "Any" || part == "All" {
			all = true
			continue
		}
		z, known := parseZone(part)
		if !known {
			ok = false
			continue
		}
		if !zoneIn(zones, z) {
			zones = append(zones, z)
		}
	}
	return zones, all, ok
}

func zoneIn(zones []state.Zone, want state.Zone) bool {
	for _, z := range zones {
		if z == want {
			return true
		}
	}
	return false
}

func effChangeZone(h Host, c *Ctx, sa *cards.SA) {
	to := ParseZone(sa.Params["Destination"])
	var originZones []state.Zone
	var originAll bool
	if from, present := sa.Params["Origin"]; present {
		var valid bool
		originZones, originAll, valid = ParseZones(from)
		if !valid {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unrecognised ChangeZone Origin " + from})
			return
		}
		// A ChangeZone from exactly Library is a hidden-zone search, not a
		// movement of objects already named by Defined$. The searching player
		// may fail to find a card with the stated quality (Min is always zero),
		// and the answer resumes this same effect before its SubAbility runs.
		// Other origins keep the existing public-zone/object path below.
		if len(originZones) == 1 && originZones[0] == state.ZLibrary && !originAll {
			effSearchLibrary(h, c, sa, to)
			return
		}
	}
	// WithCountersType$/WithCountersAmount$ make the move put counters on the
	// permanent it lands on the battlefield with -- the Undying expansion's
	// "return to the battlefield with a +1/+1 counter" (cards/keywords.go). The
	// CounterChange is emitted AFTER the MoveZone, so it lands on the moved
	// (new) object's back at its destination, exactly as Move waiting to run
	// first would want, and the counter survives onto the permanent because it
	// is added post-move. Counter (not the Move carrying it along) is what
	// keeps events/apply.go's Move from knowing anything about counters.
	withKind := sa.Params["WithCountersType"]
	withAmt := int32(1)
	if v := strings.TrimSpace(sa.Params["WithCountersAmount"]); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			// Malformed WithCountersAmount must be loud, not silently default to
			// 1 (the reviewer's item): a wrong counter count on a Returning
			// permanent is a hard-to-spot board-shape bug. A Note event (the way
			// Resolve surfaces an unimplemented API) keeps this deterministic and
			// replay-log-visible rather than dropping to a log line the event log
			// cannot account for. The movement still proceeds with the safe
			// default 1.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "malformed WithCountersAmount " + v})
		} else {
			withAmt = int32(n)
		}
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		// Origin$, when given, is a precondition: the object must actually be
		// where the script expects, or the movement does not happen. This is
		// also this build's only CR 608.2b guard for ChangeZone: a target
		// moved away by an earlier effect in the same resolution, or by a
		// response that has already resolved, is simply skipped rather than
		// moved a second time or moved from the wrong zone.
		if _, present := sa.Params["Origin"]; present && !originAll && !zoneIn(originZones, o.Zone) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: to})
		// RememberChanged$ True (Forge's spelling on the ChangeZone in the
		// Flickerwisp delayed-trigger family): the moved object joins the
		// ability's Remembered, so a DelayedTrigger that runs as a later
		// SubAbility of this same chain captures it (TrigBounce's Defined$
		// DelayTriggerRememberedLKI resolves against it when the delayed
		// trigger fires). The value is a parameter of the ongoing resolution
		// (Ctx), not game state, so mutating it here is fine -- the recall
		// is persisted into the DelayedRegister event, not written to state
		// directly.
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
		}
		if withKind != "" && to == state.ZBattlefield {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: withKind, Amount: withAmt})
		}
	}
}

// effSearchLibrary implements the hidden-origin ChangeZone shape. The option
// list is rebuilt deterministically from library order and ChangeType$, while
// the answer is carried only as option indices and object ids through the
// ordinary KChoose/resume mechanism.
//
// This round deliberately handles only the first resolved library when
// DefinedPlayer$/Defined$ names several players. A single effect cannot yet
// persist its place in a multi-player loop across more than one suspended ask;
// restarting the primitive would otherwise re-ask the first library. The
// narrowing and its measured corpus population are recorded in AGENTS.md.
func effSearchLibrary(h Host, c *Ctx, sa *cards.SA, to state.Zone) {
	players := searchPlayers(h, c, sa)
	if len(players) == 0 {
		return
	}
	owner := players[0]
	g := h.Game()
	lib := zoneOf(g, state.ZLibrary, owner)

	if c.SearchDone {
		chosen := append([]state.ObjID(nil), c.Search...)
		// Scope the answer to this primitive. Any asking primitive reached by
		// the SubAbility chain must pose its own decision.
		c.Search, c.SearchDone = nil, false
		applyLibrarySearch(h, c, sa, owner, to, chosen)
		return
	}

	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	eligible := make([]state.ObjID, 0, len(lib))
	for _, id := range lib {
		if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
			eligible = append(eligible, id)
		}
	}
	max := Num(h, c, sa, "ChangeNum", 1)
	if max < 0 {
		max = 0
	}
	if max > int32(len(eligible)) {
		max = int32(len(eligible))
	}
	// CR 701.23b/701.23d decide the minimum: a search whose card filter states
	// only a quantity must find that many (or as many as the zone holds), so
	// Min is forced up to Max; a stated-quality search keeps the fail-to-find
	// allowance of Min 0. `max` is already clamped to the number of eligible
	// cards, so a quantity-only search never asks for more than the library
	// holds (701.23d's "as many as possible"). This is a property of the
	// filter, not of Forge's Mandatory$ parameter.
	min := int32(0)
	if !SearchStatesQuality(spec) {
		min = max
	}
	// The prompt must not offer a choice the decision will refuse. A
	// quantity-only search has Min == Max, so "up to" would be a lie the
	// player only discovers when their answer is rejected.
	count := strconv.Itoa(int(max))
	prompt := "Search a library: choose up to " + count + " card(s)"
	if min == max {
		prompt = "Search a library: choose " + count + " card(s)"
	}
	chooser := searchChooser(h, c, sa)
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
		Min: int(min), Max: int(max), Source: c.Source,
		ResumeKind: "search", ResumeSA: sa,
		Prompt: prompt}
	for _, id := range eligible {
		name := "a card"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "search", Label: name, Obj: id, Player: owner})
	}
	if h.Ask(d) {
		return
	}
	// R-9: a host without a decision channel cannot ask a player, so it
	// supplies a deterministic answer in the player's place. For a
	// quantity-only search (CR 701.23d) the decision would refuse to find
	// fewer than Min cards, so the stand-in takes the first Min eligible
	// cards -- in the same ordered eligible list the decision's options
	// were built from -- or all of them when the library holds fewer
	// (701.23d's "as many as possible"). For a stated-quality search
	// (CR 701.23b) finding nothing is a legitimate fail-to-find, so the
	// stand-in still finds nothing, exactly as before. Either way the
	// search's unconditional shuffle still happens.
	var picked []state.ObjID
	if !SearchStatesQuality(spec) {
		n := int(min)
		if n > len(eligible) {
			n = len(eligible)
		}
		if n > 0 {
			picked = append(picked, eligible[:n]...)
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
			Text: "finds " + strconv.Itoa(n) + " card(s) (no engine host to ask)"})
	} else {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
			Text: "finds no card (no engine host to ask)"})
	}
	applyLibrarySearch(h, c, sa, owner, to, picked)
}

// searchPlayers resolves whose library is searched. DefinedPlayer$ takes
// precedence over Defined$; with neither, the source controller searches.
func searchPlayers(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	spec, explicit := sa.Params["DefinedPlayer"]
	if !explicit {
		spec, explicit = sa.Params["Defined"]
	}
	if !explicit || strings.TrimSpace(spec) == "" {
		return []state.PlayerID{c.Controller}
	}

	var targets []state.Target
	switch spec {
	case "RememberedController":
		for _, t := range c.Remembered {
			if t.IsPlayer {
				targets = append(targets, t)
			} else if o := h.Game().Obj(t.Obj); o != nil {
				targets = append(targets, state.Target{Player: o.Controller, IsPlayer: true})
			}
		}
	default:
		// Defined only reads the Defined key, so a tiny temporary SA lets this
		// helper share its deterministic selector grammar without mutating the
		// immutable compiled SA.
		targets = Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})
	}
	seen := make(map[state.PlayerID]bool)
	out := make([]state.PlayerID, 0, len(targets))
	for _, t := range targets {
		p := PlayerOf(h, c, t)
		if int(p) >= len(h.Game().Players) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// searchChooser resolves who answers the search prompt. You is the default;
// Targeted uses the first chosen target, and Opponent uses the first living
// opponent in deterministic turn order.
func searchChooser(h Host, c *Ctx, sa *cards.SA) state.PlayerID {
	switch sa.Params["Chooser"] {
	case "Targeted":
		if len(c.Targets) > 0 {
			return PlayerOf(h, c, c.Targets[0])
		}
	case "Opponent":
		for _, p := range h.Game().AliveFrom(c.Controller) {
			if p != c.Controller {
				return p
			}
		}
	}
	return c.Controller
}

func applyLibrarySearch(h Host, c *Ctx, sa *cards.SA, owner state.PlayerID, to state.Zone, chosen []state.ObjID) {
	g := h.Game()
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	moved := make([]state.ObjID, 0, len(chosen))
	for _, id := range chosen {
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZLibrary || o.Owner != owner ||
			!MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZLibrary, To: to, Player: owner})
		moved = append(moved, id)
		if to == state.ZBattlefield && sa.Params["WithCountersType"] != "" {
			amount := int32(1)
			if raw := strings.TrimSpace(sa.Params["WithCountersAmount"]); raw != "" {
				n, err := strconv.Atoi(raw)
				if err != nil {
					h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
						Text: "malformed WithCountersAmount " + raw})
				} else {
					amount = int32(n)
				}
			}
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id,
				Counter: sa.Params["WithCountersType"], Amount: amount})
		}
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
		if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
			h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: owner})
		}
	}

	shuffle := !strings.EqualFold(sa.Params["NoShuffle"], "True") &&
		!strings.EqualFold(sa.Params["Shuffle"], "False")
	if shuffle {
		order := append([]state.ObjID(nil), g.Zone(state.ZLibrary, owner)...)
		for i := len(order) - 1; i > 0; i-- {
			j := h.Rand(i + 1)
			order[i], order[j] = order[j], order[i]
		}
		h.Emit(events.Event{Kind: events.Shuffle, Player: owner, IDs: order, Secret: true})
	}

	// "Shuffle, then put that card on top" tutors need the placement after
	// the randomisation. MoveZone library->library first records the selected
	// cards in answer order; this one LibraryOrder makes position 0/-1 exact.
	position := strings.TrimSpace(sa.Params["LibraryPosition"])
	if to != state.ZLibrary || len(moved) == 0 || (position != "0" && position != "-1") {
		return
	}
	selected := make(map[state.ObjID]bool, len(moved))
	for _, id := range moved {
		selected[id] = true
	}
	lib := g.Zone(state.ZLibrary, owner)
	rest := make([]state.ObjID, 0, len(lib)-len(moved))
	placed := make([]state.ObjID, 0, len(moved))
	for _, id := range moved {
		if containsID(lib, id) {
			placed = append(placed, id)
		}
	}
	for _, id := range lib {
		if !selected[id] {
			rest = append(rest, id)
		}
	}
	order := make([]state.ObjID, 0, len(lib))
	if position == "0" {
		order = append(order, placed...)
		order = append(order, rest...)
	} else {
		order = append(order, rest...)
		order = append(order, placed...)
	}
	h.Emit(events.Event{Kind: events.LibraryOrder, Player: owner, IDs: order, Secret: true})
}

func effChangeZoneAll(h Host, c *Ctx, sa *cards.SA) {
	from, all, valid := ParseZones(sa.Params["Origin"])
	if !valid {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognised ChangeZoneAll Origin " + sa.Params["Origin"]})
		return
	}
	if all {
		from = []state.Zone{
			state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard,
			state.ZExile, state.ZStack, state.ZCommand, state.ZCeased,
		}
	}
	to := ParseZone(sa.Params["Destination"])
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	g := h.Game()
	for _, z := range from {
		for _, p := range g.AliveFrom(0) {
			// Snapshot the zone: emitting move events mutates it underneath us.
			ids := append([]state.ObjID(nil), g.Zone(z, p)...)
			for _, id := range ids {
				if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
				}
			}
		}
	}
}

// effDestroy is a single-target removal effect: exactly the shape CR 608.2b
// target rechecking exists for. Today the only recheck is "does the target
// still exist, and is it still on the battlefield" -- a target that stayed on
// the battlefield but became newly ineligible some other way (e.g. it gained
// Indestructible in response, or protection from the source) between
// targeting and resolution is not rechecked. See the Task 18 report.
func effDestroy(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if h.HasKeyword(o.ID, "Indestructible") {
			continue
		}
		// NoRegen$ is compared against "True", not against empty: an explicit
		// NoRegen$ False PERMITS regeneration, and reading it as "set, so
		// suppress" would invert the card. The corpus splits 144 True / 1
		// False (creepy_doll.txt), and that one is unreachable today because
		// cards/link.go auto-links only SubAbility$, not the WinSubAbility$ it
		// hangs off -- so this is correctness insurance for when that changes,
		// not a live fix.
		if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, o.ID) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
}

func effDestroyAll(h Host, c *Ctx, sa *cards.SA) {
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if h.HasKeyword(id, "Indestructible") {
				continue
			}
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				// NoRegen$ != "True", not == "": see effDestroy above.
				if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, id) {
					continue
				}
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
			}
		}
	}
}

// effSacrifice moves permanents to the graveyard. Sacrifice ignores
// Indestructible: sacrificing is not destruction (CR 701.16), so no
// HasKeyword/Indestructible gate and no ReplaceDestruction/regeneration
// consultation -- a regenerated creature does not survive being sacrificed.
// Same CR 608.2b caveat as effDestroy: only existence-and-zone is rechecked.
func effSacrifice(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	// SacValid$ narrows WHAT may be sacrificed ("Creature.nonToken",
	// "Artifact"). With no SacValid$ at all the default is "Permanent" (any
	// permanent). The older justification -- that the self-sacrifice and
	// at-end-of-step lines need "Permanent" because the object they sacrifice
	// may be an artifact, a land or a creature -- is empirically false: those
	// lines carry no Defined$ and no ValidTgts$, so Defined() resolves them
	// to the SOURCE object (effects/context.go) and they take the object-target
	// path below, where spec is never consulted at all. Measured at the corpus
	// pin (for the command, see the sc1b report): of 892 Sacrifice SAs, 328
	// carry no SacValid$; 327 of those resolve to an object (or an inherited
	// target) and never reach the default, and exactly one -- Expert-Level
	// Safe's DB$ Sacrifice | Defined$ You | ValidCard$ Card.Self -- reaches it.
	// So "Permanent" is a harmless default rather than a correct reading of
	// the corpus, and no player-targeted line carries SacValid$ Self. (That one
	// reachable line means its controller hands over whichever permanent sits
	// first in zone order -- for Expert-Level Safe, "this artifact" -- instead
	// of the no-op before this fix; see AGENTS.md.)
	spec := sa.Params["SacValid"]
	if spec == "" {
		spec = "Permanent"
	}
	// RememberSacrificed$ True drives the task's effect-driven sacrifice
	// capture: it makes effSacrifice record the LKI snapshot (power,
	// toughness, mana value) of each object it sacrifices, so a SubAbility$
	// chained after it can resolve Sacrificed$CardPower/CardManaCost/Amount
	// against what THIS ability just sacrificed. Without the flag nothing is
	// remembered -- and nothing is, because the flag is read nowhere else in
	// this package (the sacrifice_audit test only counts its occurrence), so
	// the absence is the conservative same-as-before no-op, not a regression.
	remember := sa.Params["RememberSacrificed"] != ""
	// rememberLKICapture captures the sacrificed object's LKI (before the
	// MoveZone resets its counters) into c.Sacrificed, when the flag asks it
	// to. Idempotent per call site; called exactly once per sacrificed object.
	rememberLKICapture := func(id state.ObjID) {
		if remember {
			c.Sacrificed = append(c.Sacrificed, state.SacrificedInfoOf(g, id))
		}
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			// Bounds guard: g.Zone indexes g.zones[zoneIndex(z, p)] and
			// zoneIndex has no bounds check, so an out-of-range target-supplied
			// player id would panic with "index out of range" and halt the
			// table. Player targets normally come from askTarget or AliveFrom
			// and are bounded, but the package's idiom (see cardflow.go and
			// count.go) is not to trust a target blindly.
			if int(t.Player) >= len(g.Players) {
				continue
			}
			// A sacrifice aimed at a player: that player sacrifices one
			// matching permanent. Real Magic has the player choose; this
			// engine does not ask (the mid-resolution ask machinery is being
			// reworked elsewhere), so the stand-in is deterministic and
			// replay-stable: the first permanent in battlefield order that
			// satisfies SacValid$. "You" in the spec is the sacrificing
			// player, since they choose from their own permanents.
			ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, t.Player)...)
			for _, id := range ids {
				if MatchesSpecCtx(g, spec, id, c.SpecContext(t.Player)) {
					rememberLKICapture(id)
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
					break
				}
			}
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		// A specific object target is sacrificed as-is: the choice of which
		// object was already made by the effect's targeting, so SacValid$'
		// "which one may be sacrificed" step does not re-filter a concrete
		// object (and would misfire on the corpus's SacValid$ Self lines,
		// where "Self" is not a type the filter grammar knows).
		rememberLKICapture(o.ID)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
}
