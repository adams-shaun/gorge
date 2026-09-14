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
			// Forge treats a Defined$ that resolves to objects in a hidden
			// library as the already-selected fetch list, not as the owner of a
			// fresh whole-library search. This is structural rather than keyed to
			// Remembered: ChosenCard, TopOfLibrary once resolved, and future
			// object-valued Defined selectors share the same dispatcher.
			if moveDefinedLibraryObjects(h, c, sa, to) {
				return
			}
			effSearchLibrary(h, c, sa, to)
			return
		}
		// A ChangeZone from exactly Hand with a ChangeType$ card filter and no
		// object selector is Forge's "choose N cards matching ChangeType$ from
		// your hand" shape (Burgeoning: "you may put a land card from your hand
		// onto the battlefield"). With no Defined$/DefinedPlayer$/ValidTgts$
		// the object path below would resolve Defined to the SOURCE default and
		// then skip every candidate on the Origin$ precondition -- the silent
		// no-op the handmove1 fix replaces with a real hand choice.
		// DefinedPlayer$-bearing lines name another player's hand and stay on
		// the (broken) object path until the per-player follow-up lands; a
		// ValidTgts$-bearing line names real targets the object path moves; and
		// a non-literal ChangeNum$ (an SVar name or inline Count$, ~45 raw
		// lines) is a scoped-out follow-up that also stays on that old path.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["ChangeType"] != "" && sa.Params["Defined"] == "" &&
			sa.Params["DefinedPlayer"] == "" && sa.Params["ValidTgts"] == "" {
			if _, literal := handChangeNum(sa); literal {
				effChangeZoneHand(h, c, sa, to)
				return
			}
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
	var withAmt int32
	// WithCounters* only takes effect when the object enters the battlefield.
	// Parsing a dynamic/malformed amount emits a Note, so do not parse it for
	// another destination where no CounterChange can ever be emitted.
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
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
		settleChangeZoneMove(h, c, sa, o.ID, o.Zone, to, withKind, withAmt)
	}
}

// settleChangeZoneMove is the one settle path every ChangeZone mover shares:
// the MoveZone itself, then RememberChanged$ (the moved object joins the
// ability's Remembered -- a DelayedTrigger running as a later SubAbility of
// the same chain captures it, and the value is a parameter of the ongoing
// resolution (Ctx), not game state, so mutating it here is fine), then the
// WithCountersType$/WithCountersAmount$ entry counters when the move lands on
// the battlefield. Keeping the object path and the hand-choice path on this
// one helper means the two cannot drift apart on any of the three.
func settleChangeZoneMove(h Host, c *Ctx, sa *cards.SA, id state.ObjID, from, to state.Zone, withKind string, withAmt int32) {
	h.Emit(moveZoneEvent(c, id, from, to))
	if strings.EqualFold(sa.Params["RememberChanged"], "True") {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
	}
	if withKind != "" && to == state.ZBattlefield {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: withKind, Amount: withAmt})
	}
}

// handChangeNum reads the SA's ChangeNum$ as a plain integer literal
// (absent = 1, Forge's ChangeZoneEffect default for this shape). The second
// return is false for anything else -- a non-integer, negative, or value
// outside a decision count's signed 32-bit range -- and the caller routes
// that SA to the pre-existing object path
// instead: evaluating SVar/Count$ count expressions here is a scoped-out
// follow-up, not part of handmove1.
func handChangeNum(sa *cards.SA) (int32, bool) {
	v, present := sa.Params["ChangeNum"]
	if !present || v == "" {
		return 1, true
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n < 0 {
		return 0, false
	}
	return int32(n), true
}

// effChangeZoneHand implements Forge's "choose N cards matching ChangeType$
// from your hand" ChangeZone shape (handmove1): Origin$ Hand, a ChangeType$
// card filter, ChangeNum$ cards, no object selector. The eligible pool is the
// resolving controller's own hand, rebuilt deterministically in hand order
// through the same MatchesSpecCtx resolver the library search uses; the move
// itself goes through the ordinary object mover so RememberChanged$ and any
// Destination$ behave exactly as they do everywhere else.
//
// The ask (the dig1/effDiscard strict-supersets rule): when the hand holds
// STRICTLY more ChangeType$-eligible cards than ChangeNum, the pick is a real
// KChoose posed to the controller -- Min ChangeNum, Max ChangeNum -- so a
// decision nobody could answer differently is never emitted; with ChangeNum
// or fewer eligible cards they all move deterministically, no ask. With ZERO
// eligible cards the effect resolves doing nothing (Burgeoning with no land
// in hand is a legitimate no-op). The chooser's own hand is the option pool,
// so no Secret Note/redaction is needed beyond the ordinary
// decision-attaches-only-to-its-Player projection rule.
//
// ChangeNum$ defaults to 1 when absent -- Forge's own ChangeZoneEffect
// defaults the count to one card (ChangeNum$ absent reads as "a card" in
// every text this shape carries: Burgeoning, Elvish Pioneer, Kami of Bamboo
// Groves), and the corpus's 70 absent-ChangeNum$ exact-Hand lines are all
// singular-take texts. Only a PLAIN INTEGER literal ChangeNum$ reaches this
// path: the routing guard in effChangeZone leaves a non-literal value (an
// SVar name or inline Count$, ~45 raw lines) on the pre-existing object
// path, so this function never sees one and no count expression is
// evaluated here (a scoped-out follow-up).
//
// The answer re-enters through ResumeKind "hand_move" with Ctx.HandMove /
// HandMoveDone set (rules/resolution.go); both are captured and cleared at
// the top of this walk (the fx42 scoping discipline), so a nested hand-move
// ask in the same SubAbility$ chain poses its own decision instead of
// inheriting the outer answer. A host that cannot ask (the fuzz/no-engine
// stand-in, R-9) takes the first ChangeNum eligible cards in the decision's
// own deterministic option order, with the Note that records why the richer
// path did not run. Still unread here, each a scoped-out follow-up: Tapped$
// True (entry-tapped, unread on the object path too), Optional$
// True on the ChangeZone itself (23 raw lines -- the optional take is asked
// as mandatory), Destination$ Hand/Sideboard oddities (2 lines), and any
// ConditionPresent$/ConditionDefined$ gate (the engine-wide Condition* gap).
func effChangeZoneHand(h Host, c *Ctx, sa *cards.SA, to state.Zone) {
	spec := sa.Params["ChangeType"]
	g := h.Game()
	hand := zoneOf(g, state.ZHand, c.Controller)
	// fx42 scoping: capture and clear the answered pick BEFORE anything else,
	// so a nested hand-move ask below cannot inherit it.
	ans := c.HandMove
	done := c.HandMoveDone
	c.HandMove, c.HandMoveDone = nil, false
	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	// Match the object path: WithCounters* has no effect away from the
	// battlefield, and parsing a dynamic amount there must not emit a Note.
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	// settleHandMove settles one chosen card: exactly the shared ChangeZone
	// mover. Tapped$ True is deliberately NOT read here -- it is unread on
	// the object path too, a pre-existing gap recorded as a follow-up, not
	// something to fix on this path alone.
	settleHandMove := func(id state.ObjID) {
		settleChangeZoneMove(h, c, sa, id, state.ZHand, to, withKind, withAmt)
	}
	if done {
		// Re-entry: move exactly the answered cards that still sit in the
		// controller's hand and still match the filter (a stray answer must
		// not move an object that left the hand meanwhile), in the player's
		// answer order.
		for _, id := range ans {
			if !containsID(hand, id) {
				continue
			}
			o := g.Obj(id)
			if o == nil || o.Zone != state.ZHand {
				continue
			}
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			settleHandMove(id)
		}
		return
	}
	n, _ := handChangeNum(sa)
	eligible := make([]state.ObjID, 0, len(hand))
	for _, id := range hand {
		if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
			eligible = append(eligible, id)
		}
	}
	if len(eligible) == 0 {
		return // a legitimate no-op: nothing matching in hand.
	}
	if int32(len(eligible)) <= n {
		// No choice to ask about: every eligible card moves, deterministically,
		// in hand order. No new decision of any kind.
		for _, id := range eligible {
			settleHandMove(id)
		}
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
		Min: int(n), Max: int(n), Source: c.Source,
		ResumeKind: "hand_move", ResumeSA: sa,
		Prompt: "Choose " + strconv.Itoa(int(n)) + " matching card(s) from your hand: they move to " + handDestPhrase(to)}
	for _, id := range eligible {
		name := "a card"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "hand_move", Label: name, Obj: id, Player: c.Controller})
	}
	// The shared ask boundary (effects.Ask): a ChangeNum$ 0 pick over a
	// nonempty eligible hand is Min == Max == 0 -- the empty-answer-only
	// shape -- so it is never posted; AskEmpty resolves silently through the
	// stand-in below, which moves zero cards.
	oc := Ask(h, d)
	if oc == AskAsked {
		return // resolution suspended; the answer re-enters with Ctx.HandMove set.
	}
	// R-9: a host without a decision channel cannot ask a player, so it
	// supplies the deterministic answer in the player's place -- the first
	// ChangeNum eligible cards in the same ordered eligible list the
	// decision's options were built from.
	if oc == AskNoHost {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "moves the first matching card(s) from hand (no engine host to ask)"})
	}
	for i := int32(0); i < n && i < int32(len(eligible)); i++ {
		settleHandMove(eligible[i])
	}
}

// handDestPhrase names the hand-move destination in the human-readable
// prompt; its own vocabulary so it cannot drift into digDestPhrase's or
// destinationPhrase's.
func handDestPhrase(to state.Zone) string {
	switch to {
	case state.ZBattlefield:
		return "the battlefield"
	case state.ZGraveyard:
		return "the graveyard"
	case state.ZExile:
		return "exile"
	case state.ZLibrary:
		return "the library"
	case state.ZHand:
		return "the hand"
	default:
		return "its destination"
	}
}

// withCounterAmount parses WithCountersAmount$ (default 1). Malformed values
// must be loud, not silently default to 1 (the reviewer's item): a wrong
// counter count on a Returning permanent is a hard-to-spot board-shape bug. A
// Note event (the way Resolve surfaces an unimplemented API) keeps this
// deterministic and replay-log-visible rather than dropping to a log line the
// event log cannot account for. The movement still proceeds with the safe
// default 1.
func withCounterAmount(h Host, c *Ctx, sa *cards.SA) int32 {
	v := strings.TrimSpace(sa.Params["WithCountersAmount"])
	if v == "" {
		return 1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "malformed WithCountersAmount " + v})
		return 1
	}
	return int32(n)
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
	// An empty choice is not a choice: asking it suspends a real engine host
	// until it submits an empty answer, even though no answer can differ.
	// Complete the fail-to-find directly (including its required shuffle).
	if min == 0 && max == 0 {
		// A submitted search answer resumes in a fresh Ctx, so remembered
		// objects do not leak into its SubAbility chain. Preserve that existing
		// continuation contract while omitting the otherwise meaningless ask.
		c.Remembered = nil
		applyLibrarySearch(h, c, sa, owner, to, nil)
		return
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
		// A nested hidden search resumes in the same resolution, not from a
		// blank spell context. The fetch list built by a preceding search is
		// therefore available to Card.IsRemembered and Defined$ Remembered in
		// the rest of this chain.
		ResumeRemembered: append([]state.Target(nil), c.Remembered...),
		Prompt:           prompt}
	for _, id := range eligible {
		name := "a card"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "search", Label: name, Obj: id, Player: owner})
	}
	// The shared ask boundary (effects.Ask) refuses to post a decision whose
	// only legal answer is the empty one -- with zero eligible cards max
	// clamps to 0 and a stated-quality search's Min is already 0, so that is
	// exactly the Squadron Hawk fail-to-find shape that used to soft-lock the
	// game. AskEmpty resolves it silently through the stand-in below: the
	// search still shuffles, and a fail-to-find is legitimate under
	// CR 701.23b, so nothing is degraded and no R-9 Note is recorded.
	oc := Ask(h, d)
	if oc == AskAsked {
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
	// search's unconditional shuffle still happens. An AskEmpty run takes
	// the same stand-in silently (no Note): skipping the ask is the correct
	// resolution, not a degradation.
	var picked []state.ObjID
	if !SearchStatesQuality(spec) {
		n := int(min)
		if n > len(eligible) {
			n = len(eligible)
		}
		if n > 0 {
			picked = append(picked, eligible[:n]...)
		}
		if oc == AskNoHost {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
				Text: "finds " + strconv.Itoa(n) + " card(s) (no engine host to ask)"})
		}
	} else if oc == AskNoHost {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
			Text: "finds no card (no engine host to ask)"})
	}
	applyLibrarySearch(h, c, sa, owner, to, picked)
}

// libraryFetch is one owner and the direct-library objects moved for them.
// The slice stays in Defined$ order; a map would make emitted move/shuffle
// events nondeterministic.
type libraryFetch struct {
	owner state.PlayerID
	ids   []state.ObjID
}

// moveDefinedLibraryObjects implements Forge's hidden-origin Defined$ fetch
// list. When Defined$ resolves to object(s), those identities are the list to
// move; they do NOT select a library owner for a new search. As with the
// ordinary object path, ChangeType$ does not re-filter an already named
// object. This covers Remembered, ChosenCard, TopOfLibrary, BottomOfLibrary,
// and every future object-valued Defined selector through the same dispatch.
//
// Object selectors from Hand and Graveyard already use effChangeZone's normal
// object path. Library is the exceptional origin because it otherwise enters
// effSearchLibrary. A Defined$ yielding only player targets still belongs to
// the search-owner path below. Each touched owner is shuffled once, even when
// another sub-effect already moved every fetched object: Nissa's Pilgrimage's
// final fetch-list step is the script's shuffle point after its chosen Forest
// entered the battlefield.
//
// Optional$ True is a choice over the whole known fetch list, not permission
// to silently move it. Kenessos's DBBottom is the corpus example: after its
// player declines to put the revealed card onto the battlefield, they may put
// that card on the bottom. The yes/no decision suspends before either a move
// or a shuffle; its answer is scoped in Ctx so a nested optional fetch cannot
// inherit it. A no-host run keeps the previous deterministic mover (yes), the
// R-9 fallback used by the other optional mid-resolution effects.
//
// An unrecognised Defined$ selector is a fail-closed no-op here. In
// particular it must not pass through Defined's public source fallback: that
// fallback would make an unknown selector look like an object fetch list and
// silently consume the hidden-origin effect. A resolved player target takes
// the ordinary search-owner path below, regardless of which selector yielded
// it; the target kind, not a closed spelling list, defines the role.
func moveDefinedLibraryObjects(h Host, c *Ctx, sa *cards.SA, to state.Zone) bool {
	if strings.TrimSpace(sa.Params["Defined"]) == "" {
		return false
	}
	if _, owner := sa.Params["DefinedPlayer"]; owner {
		return false
	}
	g := h.Game()
	targets, known := knownDefinedTargets(h, c, sa.Params["Defined"])
	if !known {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognised Defined library fetch " + sa.Params["Defined"]})
		return true
	}
	var fetches []libraryFetch
	objectList := false
	playerList := false
	addOwner := func(p state.PlayerID) int {
		for i := range fetches {
			if fetches[i].owner == p {
				return i
			}
		}
		fetches = append(fetches, libraryFetch{owner: p})
		return len(fetches) - 1
	}
	for _, t := range targets {
		if t.IsPlayer {
			playerList = true
			continue
		}
		objectList = true
		o := g.Obj(t.Obj)
		if o == nil || int(o.Owner) >= len(g.Players) {
			continue
		}
		i := addOwner(o.Owner)
		if o.Zone == state.ZLibrary {
			fetches[i].ids = append(fetches[i].ids, o.ID)
		}
	}
	if !objectList {
		// A resolved player list identifies whose library to search. Deriving
		// that role from the resolved target kind covers every selector with a
		// player binding (Remembered, Targeted, ChosenPlayer, and future ones),
		// instead of losing an unlisted spelling to a direct-fetch no-op. An
		// empty object fetch (an empty Remembered/ChosenCard list or an empty
		// library's TopOfLibrary) is still a direct fetch and must not degrade
		// to a fresh whole-library search.
		return !playerList
	}

	optional := strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True")
	answer := c.DefinedLibraryMove
	c.DefinedLibraryMove = "" // fx42 scoping: a nested fetch asks for itself.
	if optional && answer == "" {
		prompt := strings.TrimSpace(sa.Params["OptionalPrompt"])
		if prompt == "" {
			prompt = "Move the selected card(s)?"
		}
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
			Source: c.Source, ResumeKind: "defined_library_optional", ResumeSA: sa,
			ResumeRemembered: append([]state.Target(nil), c.Remembered...), Prompt: prompt,
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Yes", Player: c.Controller},
				{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
			}}
		if Ask(h, d) == AskAsked {
			return true
		}
		// AskNoHost cannot represent a decline. Preserve the prior direct-move
		// fallback rather than leaving a headless resolution suspended.
		answer = "yes"
	}
	if optional && answer == "no" {
		return true
	}

	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	for i := range fetches {
		f := &fetches[i]
		moved := make([]state.ObjID, 0, len(f.ids))
		for _, id := range f.ids {
			o := g.Obj(id)
			// Recheck at the point of movement: a malformed or stale Defined$
			// target must not move an object from a new zone.
			if o == nil || o.Zone != state.ZLibrary || o.Owner != f.owner {
				continue
			}
			settleChangeZoneMove(h, c, sa, id, state.ZLibrary, to, withKind, withAmt)
			moved = append(moved, id)
			if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
				h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: f.owner, Text: "entered tapped"})
			}
		}
		shuffleLibrary(h, sa, f.owner)
		placeLibraryObjects(h, sa, f.owner, moved, to)
	}
	return true
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
		ev := moveZoneEvent(c, id, state.ZLibrary, to)
		ev.Player = owner
		h.Emit(ev)
		moved = append(moved, id)
		if to == state.ZBattlefield && sa.Params["WithCountersType"] != "" {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id,
				Counter: sa.Params["WithCountersType"], Amount: withCounterAmount(h, c, sa)})
		}
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
		if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
			// This establishes the object's entry state; it is not the CR
			// 701.21a event of becoming tapped. Text is part of the replayed
			// event payload, so rules can distinguish it from an ordinary Tap
			// while replay folds the same tapped state.
			h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: owner, Text: "entered tapped"})
		}
	}

	shuffleLibrary(h, sa, owner)
	placeLibraryObjects(h, sa, owner, moved, to)
}

// shuffleLibrary is shared by an ordinary hidden search and an already-known
// Defined$ fetch list. Forge's NoShuffle$/Shuffle$ controls apply to both;
// the default is a shuffle.
func shuffleLibrary(h Host, sa *cards.SA, owner state.PlayerID) {
	if strings.EqualFold(sa.Params["NoShuffle"], "True") || strings.EqualFold(sa.Params["Shuffle"], "False") {
		return
	}
	order := append([]state.ObjID(nil), h.Game().Zone(state.ZLibrary, owner)...)
	for i := len(order) - 1; i > 0; i-- {
		j := h.Rand(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	h.Emit(events.Event{Kind: events.Shuffle, Player: owner, IDs: order, Secret: true})
}

// placeLibraryObjects implements LibraryPosition$ after its source library
// was shuffled. It is shared by a searched subset and a Defined$ fetch list.
func placeLibraryObjects(h Host, sa *cards.SA, owner state.PlayerID, moved []state.ObjID, to state.Zone) {
	position := strings.TrimSpace(sa.Params["LibraryPosition"])
	if to != state.ZLibrary || len(moved) == 0 || (position != "0" && position != "-1") {
		return
	}
	selected := make(map[state.ObjID]bool, len(moved))
	for _, id := range moved {
		selected[id] = true
	}
	lib := h.Game().Zone(state.ZLibrary, owner)
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
					h.Emit(moveZoneEvent(c, id, z, to))
					// ChangeZoneAll's remembered movement is needed for the
					// exiled-with-this-source cleanup/tally shape (Valakut
					// Exploration). Other ChangeZoneAll RememberChanged forms
					// remain outside this narrow provenance feature.
					if strings.EqualFold(sa.Params["RememberChanged"], "True") &&
						strings.Contains(sa.Params["ChangeType"], "ExiledWithSource") {
						c.Remembered = append(c.Remembered, state.Target{Obj: id})
					}
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
	// Same pre-batch discipline as effDestroyAll: the targets Defined
	// resolves are destroyed as one simultaneous batch (a multi-target
	// Destroy over a lifelink Equipment and its bearer must not make the
	// bearer's LKI depend on battlefield order), so the snapshot covers all
	// of them before the first move.
	var victims []state.ObjID
	for _, t := range Defined(h, c, sa) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if h.HasKeyword(o.ID, "Indestructible") {
			continue
		}
		victims = append(victims, o.ID)
	}
	if len(victims) > 0 {
		h.BatchDepartures(victims)
		defer h.EndBatchDepartures()
	}
	for _, id := range victims {
		o := h.Game().Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		// NoRegen$ is compared against "True", not against empty: an explicit
		// NoRegen$ False PERMITS regeneration, and reading it as "set, so
		// suppress" would invert the card. The corpus splits 144 True / 1
		// False (creepy_doll.txt), and that one is unreachable today because
		// cards/link.go auto-links only SubAbility$, not the WinSubAbility$ it
		// hangs off -- so this is correctness insurance for when that changes,
		// not a live fix.
		if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, id) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
}

func effDestroyAll(h Host, c *Ctx, sa *cards.SA) {
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	g := h.Game()
	// One pre-batch victim list across every player, then ONE departure
	// snapshot, then the emit loop (CR 704.3 simultaneity, as far as the
	// sequential emit model can express it): the CR 603.10a lifelink LKI a
	// later victim's departure capture reads must be the state from
	// immediately before the FIRST move -- a destroy-all over a
	// lifelink-granting Equipment and its bearer must not make the bearer's
	// own lifelink LKI depend on battlefield order.
	var victims []state.ObjID
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if h.HasKeyword(id, "Indestructible") {
				continue
			}
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				victims = append(victims, id)
			}
		}
	}
	if len(victims) > 0 {
		h.BatchDepartures(victims)
		defer h.EndBatchDepartures()
	}
	for _, id := range victims {
		if g.Obj(id) == nil || g.Obj(id).Zone != state.ZBattlefield {
			continue
		}
		// NoRegen$ != "True", not == "": see effDestroy's note above.
		if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, id) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
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
			// Forge's RememberSacrificed$ also remembers the card, which is
			// what a following ConditionDefined$ Remembered, Remembered$Amount
			// or RememberedCard reads (Braids, Scapeshift, Victimize).
			c.Remembered = append(copyTargets(c.Remembered), state.Target{Obj: id})
		}
	}
	who := Defined(h, c, sa)
	// A Sacrifice that names neither Defined$ nor ValidTgts$ but a SacValid$
	// other than itself is Forge's default Defined$ You: its controller
	// sacrifices a matching permanent (Braids's "you may sacrifice an
	// artifact, creature, ..."). Only a SacValid$ Self/Card.Self line (or no
	// SacValid$ at all) sacrifices the source object itself. Corpus: 66 such
	// lines, which previously sacrificed the source whatever its type.
	if _, targeted := sa.Params["ValidTgts"]; !targeted && strings.TrimSpace(sa.Params["Defined"]) == "" {
		if v := strings.TrimSpace(sa.Params["SacValid"]); v != "" && v != "Self" && v != "Card.Self" {
			who = []state.Target{{Player: c.Controller, IsPlayer: true}}
		}
	}
	// Pre-batch discipline (effDestroyAll's): the objects this effect will
	// move are chosen first, ONE departure snapshot covers them all, then
	// the emit loop runs -- a sacrifice sweep over a lifelink-granting
	// Equipment and its bearer must not make the bearer's lifelink LKI
	// depend on battlefield order.
	var victims []state.ObjID
	for _, t := range who {
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
			// A player-targeted sacrifice takes Amount$ permanents (default one).
			// The engine's no-host deterministic fallback remains battlefield
			// order; it is also what makes an Annihilator trigger complete
			// without leaving a headless match suspended.
			n := 1
			if v, err := strconv.Atoi(sa.Params["Amount"]); err == nil && v >= 0 {
				n = v
			}
			ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, t.Player)...)
			eligible := make([]state.ObjID, 0, len(ids))
			for _, id := range ids {
				if MatchesSpecCtx(g, spec, id, c.SpecContext(t.Player)) {
					eligible = append(eligible, id)
				}
			}
			if n > len(eligible) {
				n = len(eligible)
			}
			chosen := c.Sacrifice
			c.Sacrifice = nil
			// Annihilator's one defending player makes this ask resumable without
			// changing the established multi-player Sacrifice fallback.
			if chosen == nil && sa.Params["Annihilator"] == "True" && len(eligible) > n {
				opts := make([]decision.Option, 0, len(eligible))
				for _, id := range eligible {
					opts = append(opts, decision.Option{Index: len(opts), Kind: "sacrifice", Obj: id, Label: g.Obj(id).Face().Name})
				}
				if h.Ask(&decision.Decision{Player: t.Player, Kind: decision.KChoose, Min: n, Max: n,
					Prompt: "Choose permanents to sacrifice", Options: opts, ResumeKind: "sacrifice", ResumeSA: sa}) {
					return
				}
			}
			if chosen == nil {
				chosen = eligible[:n]
			}
			// The chosen permanents join the batched victims below, so the
			// departure snapshot and LKI capture stay one batch (effDestroyAll's
			// discipline) whatever the sacrifice count.
			for _, id := range chosen {
				o := g.Obj(id)
				if o == nil || o.Zone != state.ZBattlefield || o.Controller != t.Player || !MatchesSpecCtx(g, spec, id, c.SpecContext(t.Player)) {
					continue
				}
				victims = append(victims, id)
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
		victims = append(victims, o.ID)
	}
	if len(victims) > 0 {
		h.BatchDepartures(victims)
		defer h.EndBatchDepartures()
	}
	for _, id := range victims {
		if g.Obj(id) == nil || g.Obj(id).Zone != state.ZBattlefield {
			continue
		}
		rememberLKICapture(id)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
}
