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

// mixedOriginIncludesHand identifies every explicit multi-zone Origin$ that
// includes Hand. Such an effect needs one origin-aware hidden-zone chooser;
// the exact-Hand and exact-Library walkers cannot safely stand in for it.
func mixedOriginIncludesHand(zones []state.Zone, all bool) bool {
	return !all && len(zones) > 1 && zoneIn(zones, state.ZHand)
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
		// A mixed origin which includes Hand needs one chooser over cards from
		// every origin. The exact-Hand handlers below cannot provide that
		// origin-aware option list, so record the gap before retaining the
		// object path for a Defined$ card that is already known. In particular,
		// a source-default mixed-origin picker (Kastral, the Windcrested) now
		// fails loudly rather than silently doing nothing. Keep this test on
		// parsed zones rather than a list of origin strings: every new
		// Hand,<other-zone> spelling takes this same visible fallback.
		if mixedOriginIncludesHand(originZones, originAll) {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "cannot choose cards from mixed ChangeZone Origin$ " + from +
					" (an origin-aware hidden-zone chooser is not implemented)"})
		}
		// A ChangeZone from exactly Hand with no object selector is Forge's
		// hidden-origin hand put-back: the chooser picks ChangeNum$ cards (a
		// ChangeType$ filter narrows the pool; its absence -- Brainstorm's "put
		// two cards from your hand on top of your library", Jace, the Mind
		// Sculptor's [0] -- offers the whole hand). With no Defined$/
		// DefinedPlayer$/ValidTgts$ the object path below would resolve Defined
		// to the SOURCE default and then skip every candidate on the Origin$
		// precondition -- the silent no-op the handmove1 fix replaces with a
		// real hand choice (the rv2b extension drops handmove1's ChangeType$
		// requirement: the whole 239-line no-selector Origin$ Hand population
		// routes here now, 19 of it untyped).
		// An SVar or inline count expression is evaluated through Num where the
		// count grammar supports it (for example Wrenn and Seven's SVar X counts
		// lands in hand). An unknown count remains loud rather than falling through
		// to the old source-default no-op: it emits a Note and moves nothing.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["Defined"] == "" &&
			sa.Params["DefinedPlayer"] == "" && sa.Params["ValidTgts"] == "" {
			if _, supported := handMoveCountOf(h, c, sa); supported {
				effChangeZoneHand(h, c, sa, to)
				return
			}
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "cannot choose ChangeNum$ " + strings.TrimSpace(sa.Params["ChangeNum"]) +
					" cards from hand (a non-literal count is not a bound this engine can evaluate)"})
			return
		}
		// A ChangeZone from exactly Hand whose hand OWNER is selected --
		// DefinedPlayer$-alone (Kynaios and Tiro's "each player may put a land
		// card from their hand onto the battlefield", Braids, Conjurer Adept,
		// Mindleech Ghoul) or ValidTgts$-alone naming the players (Karn
		// Liberated's "[+4]: Target player exiles a card from their hand",
		// Kyoki, Sanity's Eclipse) -- is the per-owner hidden-hand shape: one
		// chooser ask per hand owner, chained through the persisted
		// Ctx.HandMoveTarget cursor. The rv2b r2 finding: these shapes used to
		// fall through to the object path, where Defined() resolved to the
		// source (or to targets the Origin$ precondition then skipped because
		// they are PLAYERS, not hand cards) -- another silent no-op.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["Defined"] == "" &&
			(sa.Params["DefinedPlayer"] != "" || sa.Params["ValidTgts"] != "") {
			effChangeZoneHandOwners(h, c, sa, to)
			return
		}
		// DefinedPlayer$ alongside a Defined$ that names concrete objects
		// (Wilt-Leaf Liege's DefinedPlayer$ ReplacedPlayer + Defined$
		// ReplacedCard, 1 corpus line) keeps the object path -- the moved
		// objects are already named -- but the owner parameter is unread
		// there, so the shape is loud about it rather than silent.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["Defined"] != "" && sa.Params["DefinedPlayer"] != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "DefinedPlayer$ " + sa.Params["DefinedPlayer"] +
					" is unread next to Defined$ " + sa.Params["Defined"] +
					" (the move goes to the named objects alone)"})
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
		// Inlined rather than routed through settleChangeZoneMove: this loop
		// carries the exiled-with association and the RememberChanged$
		// event-backed rider (eventRemember) in a specific order (MoveZone,
		// exiled-with, RememberChanged, WithCounters) that predates the
		// shared settle helper, and neither is shared with that helper's
		// other callers (see settleChangeZoneMoveAs's doc comment).
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: to})
		exiledWithAssociation(h, c, o.ID, to)
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
			eventRemember(h, c, o.ID)
		}
		if withKind != "" && to == state.ZBattlefield {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: withKind, Amount: withAmt})
		}
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
	settleChangeZoneMoveAs(h, c, sa, id, from, to, withKind, withAmt, 0, false)
}

// settleChangeZoneMoveAs is the one settle path every ChangeZone mover shares
// (settleChangeZoneMove is its event-Player-unset form), plus the explicit
// event-Player form: a hidden-zone move of ANOTHER player's card carries that
// player as the event's Player -- the same attribution the library search's
// move applies -- so the view layer's hidden-card redaction sees the move the
// way the owner does. The MoveZone itself, then RememberChanged$ (the moved
// object joins the ability's Remembered -- a DelayedTrigger running as a
// later SubAbility of the same chain captures it, and the value is a
// parameter of the ongoing resolution (Ctx), not game state, so mutating it
// here is fine), then the WithCountersType$/WithCountersAmount$ entry
// counters when the move lands on the battlefield. Keeping the object path
// and the hand-choice path on this one helper means the two cannot drift
// apart on any of the three. Tapped$ True is event-backed for the hidden
// library paths, but not for a card entering from hand; before every such
// move this common path makes the narrowing replay-visible rather than
// silently entering the card untapped.
//
// The exiled-with association and the RememberChanged$ event-backed rider
// (eventRemember) are NOT done here: they are scoped to the two ORIGINAL
// ChangeZone movers that carried them before this helper existed (the
// object-target loop in effChangeZone and applyLibrarySearch's hidden-search
// mover), not to every caller of this now-shared settle path -- widening
// their scope here would move acceptance-game replay hashes beyond the
// reviewed change.
func settleChangeZoneMoveAs(h Host, c *Ctx, sa *cards.SA, id state.ObjID, from, to state.Zone, withKind string, withAmt int32, player state.PlayerID, hasPlayer bool) {
	if from == state.ZHand && to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
		notePlayer := c.Controller
		if hasPlayer {
			notePlayer = player
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: notePlayer,
			Text: "Tapped$ True on a hand ChangeZone is not implemented; the card enters untapped"})
	}
	ev := moveZoneEvent(c, id, from, to)
	if hasPlayer {
		ev.Player = player
	}
	h.Emit(ev)
	if strings.EqualFold(sa.Params["RememberChanged"], "True") {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
	}
	if withKind != "" && to == state.ZBattlefield {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: withKind, Amount: withAmt})
	}
}

// exiledWithAssociation emits Forge's ChangeZoneEffect.handleExiledWith
// association for a non-token card this effect just exiled: the host's
// distinct exiledCards collection. It is deliberately NOT an ImprintCards$
// association: DefinedCards$ ExiledWith consumes this list, while
// ImprintedController only consumes explicit ImprintCards$ entries. Scoped to
// the object-target loop and applyLibrarySearch, the two movers that carried
// this association originally.
func exiledWithAssociation(h Host, c *Ctx, id state.ObjID, to state.Zone) {
	if to != state.ZExile || c.Source == 0 {
		return
	}
	if o := h.Game().Obj(id); o != nil && !o.IsToken {
		h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: []state.ObjID{id}, Text: "exiled-with"})
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

// effChangeZoneHand is the whole-hand shape (handmove1/rv2b r1): Origin$ Hand
// with no player selector -- the resolving controller's own hand is the one
// owner, and the controller is its own chooser (Brainstorm, Jace the Mind
// Sculptor's [0], Sawtooth Loon, Burgeoning). The mechanics -- the ask gate
// (the dig1/effDiscard strict-supersets rule), explicit markers plus real
// card/script text for markerless optionality (never an assumed "may"), the
// fx42 re-entry scoping, the R-9 stand-in, the Destination$ Library placement
// (absent LibraryPosition$ = TOP in answer order; Shuffle$ True randomises instead)
// and the unread-parameter list -- are handMoveOwnersWalk's, which this
// delegates to with the one-owner, chooser==owner, no-random configuration.
// Only a literal ChangeNum$ (or its absent default 1) reaches here: the
// routing in effChangeZone Notes a non-literal before this is ever called.
func effChangeZoneHand(h Host, c *Ctx, sa *cards.SA, to state.Zone) {
	count, _ := handMoveCountOf(h, c, sa)
	handMoveOwnersWalk(h, c, sa, to, []state.PlayerID{c.Controller}, count, false, nil, false)
}

// handMoveCount is the ChangeNum$ bound one hidden-hand walk carries: either
// a fixed bound resolved once for the whole resolution, or the per-owner
// eligible count (ChangeNum$ NumInHand / HandSize -- Forge's "all matching
// cards in that hand" texts: Eradicate, Extirpate, Kotose, Lost Legacy, The
// Great Aurora).
type handMoveCount struct {
	fixed    int32
	perOwner bool
}

// handMoveCountOf classifies a hidden-hand walk's ChangeNum$. Absent reads as
// 1 (Forge's ChangeZoneEffect default). "NumInHand"/"HandSize" are the
// per-owner spellings. A plain integer literal is that literal. An SVar-named
// count (ChangeNum$ X / Y over an SVar: body) or a bare "X" (the paid X,
// CR 107.3i) resolves through the ordinary count evaluator, bound to the
// resolving context. Anything else returns false and the caller is loud (a
// Note) rather than degrading to a silent zero-count no-op.
func handMoveCountOf(h Host, c *Ctx, sa *cards.SA) (handMoveCount, bool) {
	raw := strings.TrimSpace(sa.Params["ChangeNum"])
	if raw == "" {
		return handMoveCount{fixed: 1}, true
	}
	if strings.EqualFold(raw, "NumInHand") || strings.EqualFold(raw, "HandSize") {
		return handMoveCount{perOwner: true}, true
	}
	if n, ok := handChangeNum(sa); ok {
		return handMoveCount{fixed: n}, true
	}
	resolvable := raw == "X" || strings.HasPrefix(raw, "Count$") ||
		strings.HasPrefix(raw, "Sacrificed$") || strings.HasPrefix(raw, "TriggerCount$")
	if c != nil && c.SVars != nil {
		if _, exists := c.SVars[raw]; exists {
			resolvable = true
		}
	}
	if !resolvable {
		return handMoveCount{}, false
	}
	n := Num(h, c, sa, "ChangeNum", 1)
	if n < 0 {
		n = 0
	}
	return handMoveCount{fixed: n}, true
}

// effChangeZoneHandOwners implements the owner-SELECTED hidden-hand shape
// (rv2b r2): Origin$ Hand with DefinedPlayer$-alone (Kynaios and Tiro's "each
// player may put a land card from their hand onto the battlefield", Braids,
// Conjurer Adept, Mindleech Ghoul) or ValidTgts$-alone naming the players
// whose hand moves (Karn Liberated's "[+4]: Target player exiles a card from
// their hand", Kyoki, Sanity's Eclipse). One chooser ask per hand owner,
// chained across owners through the persisted Ctx.HandMoveTarget cursor --
// the walk restarts on every answer, skips the owners already answered, and
// asks the next one -- exactly effDig's per-target continuation, but with a
// REAL ask for every later owner rather than a deterministic stand-in (the
// hand owners are few and each ask is short). Whose hand and who answers are
// the two selectors this shape carries: the owners come from
// DefinedPlayer$/ValidTgts$, the chooser from Chooser$ (Forge's default is
// the hand owner). Every shape this function cannot model emits a Note and
// moves nothing -- the finding's floor: never a silent no-op.
func effChangeZoneHandOwners(h Host, c *Ctx, sa *cards.SA, to state.Zone) {
	owners, ok := handMoveOwners(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "cannot resolve the hand owner (DefinedPlayer$ " + strings.TrimSpace(sa.Params["DefinedPlayer"]) +
				"); no hand card moves"})
		return
	}
	if len(owners) == 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "the hand owner selector names no player this engine can resolve; no hand card moves"})
		return
	}
	count, ok := handMoveCountOf(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "cannot choose ChangeNum$ " + strings.TrimSpace(sa.Params["ChangeNum"]) +
				" cards from a selected hand (a count this engine cannot evaluate)"})
		return
	}
	choosers := make([]state.PlayerID, len(owners))
	for i, owner := range owners {
		ch, ok := handMoveChooserFor(h, c, sa, owner)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Chooser$ " + strings.TrimSpace(sa.Params["Chooser"]) +
					" is not a chooser this engine can resolve; no hand card moves"})
			return
		}
		choosers[i] = ch
	}
	random := strings.EqualFold(strings.TrimSpace(sa.Params["AtRandom"]), "True")
	handMoveOwnersWalk(h, c, sa, to, owners, count, random, func(_ Host, _ *Ctx, _ *cards.SA, owner state.PlayerID) (state.PlayerID, bool) {
		return choosers[ownerIndex(owners, owner)], true
	}, true)
}

// ownerIndex is the position of owner in owners (owners is small and built
// without duplicates).
func ownerIndex(owners []state.PlayerID, owner state.PlayerID) int {
	for i, p := range owners {
		if p == owner {
			return i
		}
	}
	return 0
}

// handMoveOwners resolves whose hands an owner-selected hidden-hand ChangeZone
// moves from. DefinedPlayer$ takes precedence and resolves through the same
// deterministic selector grammar the library search uses (searchPlayers); a
// ValidTgts$-alone line's chosen targets are the hand owners. It fails
// CLOSED: a player spec this build does not model returns ok=false and the
// caller emits its loud Note -- degrading an unmodelled selector to the
// resolving controller's hand would move (and reveal) cards from the WRONG
// player's hidden hand, which is worse than moving none.
func handMoveOwners(h Host, c *Ctx, sa *cards.SA) ([]state.PlayerID, bool) {
	if spec := strings.TrimSpace(sa.Params["DefinedPlayer"]); spec != "" {
		if _, modelled := definedSpec(h, c, spec); !modelled {
			return nil, false
		}
		return searchPlayers(h, c, sa), true
	}
	// ValidTgts$-alone: Defined's own rule names the chosen targets.
	owners := make([]state.PlayerID, 0, len(c.Targets))
	seen := make(map[state.PlayerID]bool, len(c.Targets))
	for _, t := range Defined(h, c, sa) {
		if !t.IsPlayer {
			// A hand-ownership selector that resolved to a card is not a shape
			// this chooser can honour.
			return nil, false
		}
		p := PlayerOf(h, c, t)
		if int(p) >= len(h.Game().Players) || seen[p] {
			continue
		}
		seen[p] = true
		owners = append(owners, p)
	}
	return owners, true
}

// handMoveChooserFor resolves who answers one owner's hidden-hand ask.
// Forge's default for the shape is the hand owner (Kynaios and Tiro's "each
// player may put", Mindleech Ghoul's "defending player exiles a card from
// their hand"); Chooser$ You is the caster picking out of another player's
// hand (Kitesail Freebooter, Witness the End), Chooser$ Targeted the chosen
// target (Karn Liberated), and the TriggeredTarget/TriggeredPlayer spellings
// the causing event's bound player (Kheru Mind Eater, Widespread Panic). An
// unmodelled value fails closed (ok=false) so the caller is loud rather than
// handing the ask to a guessed seat.
func handMoveChooserFor(h Host, c *Ctx, sa *cards.SA, owner state.PlayerID) (state.PlayerID, bool) {
	switch strings.TrimSpace(sa.Params["Chooser"]) {
	case "", "Owner":
		return owner, true
	case "You":
		return c.Controller, true
	case "Targeted":
		if len(c.Targets) > 0 {
			return PlayerOf(h, c, c.Targets[0]), true
		}
		return owner, true
	case "TriggeredTarget":
		if c.TriggerTarget.IsPlayer {
			return c.TriggerTarget.Player, true
		}
		if c.TriggerTarget.Obj != 0 {
			if o := h.Game().Obj(c.TriggerTarget.Obj); o != nil {
				return o.Controller, true
			}
		}
		return owner, true
	case "TriggeredPlayer":
		if c.TriggerPlayer.IsPlayer {
			return c.TriggerPlayer.Player, true
		}
		return owner, true
	}
	return owner, false
}

// handMoveOwnersWalk is the ONE hidden-hand mover both shapes share: for each
// owner in order the eligible pool is that owner's hand filtered through
// ChangeType$ (absent: the whole hand), the bound is the count (perOwner:
// that pool's own size), and the pick is one of three shapes -- the chained
// ask (STRICTLY more eligible cards than the bound: a real KChoose to the
// chooser, Min 0 when the take is optional else the bound, Max the bound, the
// answer re-entering through ResumeKind "hand_move" with ResumeTarget
// binding it to this owner), the no-choice deterministic take (a REQUIRED
// move with eligible <= bound: every eligible card moves, in hand order, no
// ask), or AtRandom$'s engine-random pick (no ask: randomness, not a player
// choice, picks). An OPTIONAL move takes the choice path whenever there is
// at least one eligible card, including eligible <= bound: declining remains
// a meaningful answer even when taking every card is the only nonempty pick
// (an empty-only ChangeNum$ 0 still resolves through AskEmpty). The re-entry
// contract (fx42 scoping): Ctx.HandMove/HandMoveDone/HandMoveTarget are captured and
// cleared at the top of the walk; owners before the cursor completed before a
// later owner suspended and are skipped, the cursor's owner consumes the
// answer (moved exactly as answered, revalidated against the CURRENT hand
// and filter), and owners after it continue the chain.
//
// The whole-hand shape (handmove1/rv2b r1) is the one-owner case of this
// walk, with the chooser == the owner -- the r1 contracts (ask shape, card
// text/explicit-marker optionality, R-9 stand-in text, zero-eligible silence,
// library tail) are this walk's contracts, unchanged. Still unread here, each
// a scoped-out follow-up:
// Destination$ Hand/Sideboard oddities (2 lines), and any ConditionPresent$/
// ConditionDefined$ gate (the engine-wide Condition* gap).
func handMoveOwnersWalk(h Host, c *Ctx, sa *cards.SA, to state.Zone, owners []state.PlayerID,
	count handMoveCount, random bool, chooserFor func(Host, *Ctx, *cards.SA, state.PlayerID) (state.PlayerID, bool),
	eventPlayer bool) {
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card" // the whole hand: Brainstorm, Jace's [0], Sawtooth Loon
	}
	g := h.Game()
	// fx42 scoping: capture and clear the answered pick (and the cursor that
	// binds it to the owner that asked) BEFORE anything else, so a nested
	// hand-move ask below cannot inherit them.
	ans := c.HandMove
	done := c.HandMoveDone
	cursor := c.HandMoveTarget
	c.HandMove, c.HandMoveDone, c.HandMoveTarget = nil, false, 0
	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	// Match the object path: WithCounters* has no effect away from the
	// battlefield, and parsing a dynamic amount there must not emit a Note.
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	// settleHandMove settles one chosen card: exactly the shared ChangeZone
	// mover; on the owner-SELECTED shapes the Move event also carries the
	// hand's OWNER as its Player (a hidden-zone move of another player's
	// card -- the same attribution the library search's move carries),
	// while the whole-hand shape keeps its historical event shape
	// (eventPlayer false, the r1 golden contract). settleChangeZoneMoveAs is
	// also the one loud Tapped$ True fallback for every hand-origin mover, so
	// concrete Defined$ objects and future hand-owner selectors cannot silently
	// miss the unsupported entry state.
	settleHandMove := func(id state.ObjID, owner state.PlayerID) {
		settleChangeZoneMoveAs(h, c, sa, id, state.ZHand, to, withKind, withAmt, owner, eventPlayer)
	}
	for i, owner := range owners {
		hand := zoneOf(g, state.ZHand, owner)
		eligible := make([]state.ObjID, 0, len(hand))
		for _, id := range hand {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
			}
		}
		if done && i < cursor {
			// This owner answered on an earlier pass, before a later owner
			// suspended the walk. Re-running it could move a second batch, so
			// skip it (effDig's per-target continuation contract).
			continue
		}
		if done && i == cursor {
			// Re-entry: move exactly the answered cards that still sit in THIS
			// owner's hand and still match the filter (a stray answer must not
			// move an object that left the hand meanwhile), in the player's
			// answer order.
			var moved []state.ObjID
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
				settleHandMove(id, owner)
				moved = append(moved, id)
			}
			handLibraryTail(h, g, sa, c.Source, owner, moved, to)
			continue
		}
		n := count.fixed
		if count.perOwner {
			n = int32(len(eligible))
		}
		if len(eligible) == 0 || n == 0 {
			// No eligible card, or an empty-only ChangeNum$ 0 choice: both
			// complete silently before optionality can matter (AskEmpty's
			// shared contract).
			continue
		}
		var moved []state.ObjID
		if random {
			// AtRandom$ True: the engine picks, not a player -- ChangeNum$
			// random distinct eligible cards (corpus: always 1, mandatory),
			// through the seeded generator, so the pick replays.
			pool := append([]state.ObjID(nil), eligible...)
			for k := int32(0); k < n && len(pool) > 0; k++ {
				j := h.Rand(len(pool))
				settleHandMove(pool[j], owner)
				moved = append(moved, pool[j])
				pool = append(pool[:j], pool[j+1:]...)
			}
			handLibraryTail(h, g, sa, c.Source, owner, moved, to)
			continue
		}
		// NumInHand/HandSize means "all matching cards in that hand", an
		// intrinsically required all-cards move (Eradicate, Extirpate, The
		// Great Aurora). Its count semantics settle optionality even when the
		// script has neither marker nor explanatory text; preserve an explicit
		// Optional$ marker should a future script carry one.
		intrinsicAll := count.perOwner && strings.TrimSpace(sa.Params["Optional"]) == "" && strings.TrimSpace(sa.Params["Mandatory"]) == ""
		optional, optionalKnown := handTakeOptional(h, c, sa, to)
		if intrinsicAll {
			optional, optionalKnown = false, true
		}
		if !optionalKnown {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "cannot determine whether the markerless hand move is optional; no hand card moves"})
			return
		}
		if int32(len(eligible)) <= n && !optional {
			// A required move with no possible nonempty selection alternative
			// takes every eligible card deterministically. An OPTIONAL move
			// must still ask here: declining is a distinct, legal answer even
			// when every nonempty answer takes all eligible cards.
			for _, id := range eligible {
				settleHandMove(id, owner)
				moved = append(moved, id)
			}
			handLibraryTail(h, g, sa, c.Source, owner, moved, to)
			continue
		}
		chooser := owner
		if chooserFor != nil {
			chooser, _ = chooserFor(h, c, sa, owner)
		}
		min := int(n)
		if optional {
			min = 0 // "you may put": none is a legal answer
		}
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
			Min: min, Max: int(n), Source: c.Source,
			ResumeKind: "hand_move", ResumeSA: sa, ResumeTarget: i,
			Prompt: handMovePromptFor(sa, to, int(n), chooser == owner)}
		for _, id := range eligible {
			name := "a card"
			if o := g.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "hand_move", Label: name, Obj: id, Player: owner})
		}
		// The shared ask boundary (effects.Ask): a ChangeNum$ 0 pick over a
		// nonempty eligible hand is Min == Max == 0 -- the empty-answer-only
		// shape -- so it is never posted; AskEmpty resolves silently through
		// the stand-in below, which moves zero cards.
		oc := Ask(h, d)
		if oc == AskAsked {
			return // resolution suspended; the answer re-enters with Ctx.HandMove set.
		}
		// R-9: a host without a decision channel cannot ask a player, so it
		// supplies the deterministic answer in the player's place -- the first
		// ChangeNum eligible cards in the same ordered eligible list the
		// decision's options were built from.
		if oc == AskNoHost {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
				Text: "moves the first matching card(s) from hand (no engine host to ask)"})
		}
		for k := int32(0); k < n && int(k) < len(eligible); k++ {
			settleHandMove(eligible[k], owner)
			moved = append(moved, eligible[k])
		}
		handLibraryTail(h, g, sa, c.Source, owner, moved, to)
	}
}

// handTakeOptional reads Forge's optional-vs-mandatory markers for a
// hidden-origin hand move. A missing marker is NOT an optional default:
// Volrath's Dungeon is markerless but requires its target to put a card back.
// Forge does not encode that distinction in ChangeZone's parameters, so the
// markerless may-shapes are recognised from their card/script text (Burgeoning,
// Oviya, Volcanic Spite); text we cannot classify fails closed and loudly at
// the caller rather than granting an invented decline.
func handTakeOptional(h Host, c *Ctx, sa *cards.SA, to state.Zone) (optional, known bool) {
	if strings.EqualFold(strings.TrimSpace(sa.Params["Mandatory"]), "True") {
		return false, true
	}
	if o := strings.TrimSpace(sa.Params["Optional"]); o != "" {
		return strings.EqualFold(o, "True") || strings.EqualFold(o, "You"), true
	}
	text := sa.Params["SpellDescription"]
	if o := h.Game().Obj(c.Source); o != nil && o.Face() != nil {
		text += "\n" + o.Face().Oracle
	}
	if strings.TrimSpace(text) == "" {
		return false, false
	}
	return handMoveTextOptional(text, to)
}

// handMoveTextOptional recognises the actual English may-forms for the one
// ChangeZone move being resolved. "Put any number" is optional even without
// the word may. It requires the destination's action verb and a hand
// reference IN THE SAME sentence: an unrelated "may put" elsewhere on a
// multi-ability card is not evidence that this move may be declined.
// Conversely, text that lacks a matching action is unknown, so the caller
// emits its fail-closed Note.
func handMoveTextOptional(text string, to state.Zone) (optional, known bool) {
	text = strings.ToLower(text)
	var action string
	switch to {
	case state.ZBattlefield, state.ZLibrary:
		action = "put"
	case state.ZExile:
		action = "exile"
	case state.ZGraveyard:
		action = "discard"
	case state.ZHand:
		action = "return"
	default:
		return false, false
	}
	if handMovePhraseMentionsHand(text, "may "+action) ||
		handMovePhraseFollowsHandReveal(text, "may "+action) ||
		(to == state.ZLibrary && handMovePhraseMentionsHand(text, "may shuffle")) ||
		(handMovePhraseMentionsHand(text, "any number") && handMovePhraseMentionsHand(text, action)) {
		return true, true
	}
	if handMovePhraseSupportsRequiredMove(text, action) ||
		handMovePhraseFollowsHandChoice(text, action) ||
		(to == state.ZLibrary && handMovePhraseSupportsRequiredMove(text, "shuffle")) {
		return false, true
	}
	return false, false
}

// handMovePhraseSupportsRequiredMove also accepts "choose" in the action's
// sentence: a preceding RevealHand can make the later "You choose ... and
// exile that card" sentence omit the word hand (Thought-Knot Seer), but it is
// still an unambiguously required chooser action.
func handMovePhraseSupportsRequiredMove(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		if strings.Contains(text[begin:end], "hand") || strings.Contains(text[begin:end], "choose") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePhraseFollowsHandChoice recognises the same hidden-hand sequence
// when Forge split its selection and movement into sentences: "reveal their
// hand. You choose a card from it. Exile that card" (Kitesail Freebooter).
// It scans only the action sentence and its three predecessors, all of which
// must establish the hand -> choice -> pronoun chain.
func handMovePhraseFollowsHandChoice(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		contextStart := begin
		for n := 0; n < 3 && contextStart > 0; n++ {
			prior := strings.TrimRight(text[:contextStart], ".; ")
			if j := strings.LastIndexAny(prior, ".;"); j >= 0 {
				contextStart = j + 1
			} else {
				contextStart = 0
			}
		}
		context := text[contextStart:end]
		if (strings.Contains(text[begin:end], "that card") || strings.Contains(text[begin:end], " it")) &&
			strings.Contains(context, "hand") && strings.Contains(context, "choose") && strings.Contains(context, "it") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePhraseFollowsHandReveal recognises the usual two-sentence hidden
// hand wording: "Target player reveals their hand. You may put ... from it."
// The pronoun is enough only immediately after a hand-reveal sentence, so an
// unrelated optional action elsewhere cannot make this move optional.
func handMovePhraseFollowsHandReveal(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		prior := strings.TrimRight(text[:begin], ".; ")
		if j := strings.LastIndexAny(prior, ".;"); j >= 0 {
			prior = prior[j+1:]
		}
		if strings.Contains(prior, "hand") && strings.Contains(text[begin:end], "it") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePhraseMentionsHand keeps text classification local to the sentence
// carrying a candidate action. Forge's Oracle text uses periods for sentence
// boundaries; semicolons also separate instructions often enough to be a safe
// boundary here.
func handMovePhraseMentionsHand(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		if strings.Contains(text[i:end], "hand") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePrompt builds the human-readable ask text, naming the top/bottom
// placement when the destination is the library (the one destination where
// WHERE matters to the chooser), and whose hand it is when the chooser is
// not the hand's owner (Chooser$ You: the caster picks out of another
// player's hand).
func handMovePromptFor(sa *cards.SA, to state.Zone, n int, own bool) string {
	dest := handDestPhrase(to)
	if to == state.ZLibrary {
		if strings.TrimSpace(sa.Params["LibraryPosition"]) == "-1" {
			dest = "the bottom of your library"
		} else {
			dest = "the top of your library"
		}
	}
	whose := "your hand"
	if !own {
		whose = "that player's hand"
		if to == state.ZLibrary {
			if strings.TrimSpace(sa.Params["LibraryPosition"]) == "-1" {
				dest = "the bottom of that player's library"
			} else {
				dest = "the top of that player's library"
			}
		}
	}
	return "Choose " + strconv.Itoa(n) + " card(s) from " + whose + ": they move to " + dest
}

// handLibraryTail is the post-move library placement both hidden-origin
// movers end with. A hand put-back only shuffles when its own script says
// so (Shuffle$ True -- Slowtrip), while a library search shuffles by
// default; the placement itself is the shared libraryOrderPlacement helper,
// with Forge's absent-LibraryPosition$ default (TOP) applied for the hand
// path -- Brainstorm and Jace's [0] name no LibraryPosition$ and their
// oracle puts the cards on top.
func handLibraryTail(h Host, _ *state.Game, sa *cards.SA, source state.ObjID, owner state.PlayerID, moved []state.ObjID, to state.Zone) {
	if to != state.ZLibrary || len(moved) == 0 {
		return
	}
	// DestinationAlternative$/LibraryPositionAlternative$ (Dream Cache's "both
	// on top of your library or both on the bottom", 1 raw line) is a modal
	// destination choice this engine cannot yet ask: the alternative is named
	// in a Note and the primary destination/position is taken
	// deterministically, so the unsupported shape is never silent.
	if alt := strings.TrimSpace(sa.Params["DestinationAlternative"]); alt != "" || strings.TrimSpace(sa.Params["LibraryPositionAlternative"]) != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: source, Player: owner,
			Text: "DestinationAlternative$ " + alt + " is not a choice this engine can ask; the cards take the primary destination"})
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["Shuffle"]), "True") &&
		!strings.EqualFold(strings.TrimSpace(sa.Params["NoShuffle"]), "True") {
		shuffleLibraryExplicit(h, sa, owner)
		return // a shuffled library has no meaningful LibraryPosition$
	}
	position := strings.TrimSpace(sa.Params["LibraryPosition"])
	if position != "" && position != "0" && position != "-1" {
		h.Emit(events.Event{Kind: events.Note, Obj: source, Player: owner,
			Text: "LibraryPosition$ " + position + " is not implemented; the cards go on top"})
	}
	libraryOrderPlacement(h, owner, moved, position == "-1")
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
		// The walk's Remembered rides the ask (rules restores it on the
		// resume) so the re-entered eligibility recheck and the SubAbility$
		// after this one still see the cards RememberChanged$ captured -- a
		// cast spell's mid-resolution Remembered lives only in the resolving
		// Ctx frame, and without the ride the answer's recheck (and Nissa's
		// Pilgrimage's "one onto the battlefield" leg) would re-resolve
		// IsRemembered against an empty set and move nothing. A nested hidden
		// search resumes in the same resolution too, so the fetch list built
		// by a preceding search stays available to Card.IsRemembered and
		// Defined$ Remembered in the rest of this chain.
		ResumeRemembered: copyTargets(c.Remembered),
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
		if to == state.ZExile && c.Source != 0 {
			if o := g.Obj(id); o != nil && !o.IsToken {
				h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: []state.ObjID{id}, Text: "exiled-with"})
			}
		}
		moved = append(moved, id)
		if to == state.ZBattlefield && sa.Params["WithCountersType"] != "" {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id,
				Counter: sa.Params["WithCountersType"], Amount: withCounterAmount(h, c, sa)})
		}
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
			eventRemember(h, c, id)
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

// shuffleLibrary applies the default hidden-library shuffle used by searches
// and direct Defined$ fetches. A hand put-back is different: it shuffles only
// when its own SA explicitly says Shuffle$ True, and uses
// shuffleLibraryExplicit below.
func shuffleLibrary(h Host, sa *cards.SA, owner state.PlayerID) {
	if strings.EqualFold(sa.Params["NoShuffle"], "True") || strings.EqualFold(sa.Params["Shuffle"], "False") {
		return
	}
	shuffleLibraryOrder(h, owner)
}

func shuffleLibraryExplicit(h Host, sa *cards.SA, owner state.PlayerID) {
	if strings.EqualFold(sa.Params["Shuffle"], "True") &&
		!strings.EqualFold(sa.Params["NoShuffle"], "True") {
		shuffleLibraryOrder(h, owner)
	}
}

func shuffleLibraryOrder(h Host, owner state.PlayerID) {
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
	libraryOrderPlacement(h, owner, moved, position == "-1")
}

// libraryOrderPlacement is the one LibraryPosition$ placement both
// hidden-origin movers (the library search's tutor-back and the hand
// put-back) share. The moved cards are already in the library (Move appended
// them at the bottom, in settle order); this one Secret LibraryOrder makes
// position exact: bottom=false puts the chosen cards on TOP in chosen order,
// bottom=true leaves them at the bottom in that same order, with the rest of
// the library beneath/above them respectively. Secret so the full order is
// visible only to the library's owner (redaction rule (1)).
func libraryOrderPlacement(h Host, owner state.PlayerID, moved []state.ObjID, bottom bool) {
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
	if !bottom {
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
	// UnlessCost$ gate: an unless-pay Sacrifice ("pay or sacrifice it", or
	// Vexing Devil's inverted "any opponent may have it deal 4 damage to
	// them; if a player does, sacrifice it") asks first. When the gate
	// consumed the resolution -- an ask was posed (suspended), the answered
	// choice spares the permanent, or every opponent declined the damage
	// offer -- there is nothing to sacrifice and the body below must not run.
	if sacrificeUnlessPay(h, c, sa) {
		return
	}
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
	// Damage-replacement bodies carry the amount of the event they replace.
	// The sole corpus Sacrifice body in that class is Dralnu's "sacrifice that
	// many permanents"; consume Amount$ there without changing the broader
	// primitive's documented one-per-player stand-in outside replacement
	// resolution.
	amount := int32(1)
	if c.ReplacementAmount > 0 && sa.Params["Amount"] != "" {
		amount = Num(h, c, sa, "Amount", 1)
		if amount < 0 {
			amount = 0
		}
	}
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
			eventRemember(h, c, id)
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
			// The multi-permanent count defaults to `amount` -- 1 for an
			// ordinary Sacrifice line, or the damage-replacement Amount$
			// resolved above (Dralnu, Lich Lord's "sacrifice that many
			// permanents" DB$ ReplaceDamage body) when this call is a damage
			// replacement's redirect. The Annihilator expansion's generated
			// SA carries its own count in its Annihilator$ marker
			// (cards/keywords.go) and overrides it; the two contexts never
			// coincide in the corpus.
			n := int(amount)
			if ann := sa.Params["Annihilator"]; ann != "" {
				if v, err := strconv.Atoi(ann); err == nil && v >= 0 {
					n = v
				}
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
			if chosen == nil && sa.Params["Annihilator"] != "" && len(eligible) > n {
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

// sacrificeUnlessPay implements UnlessCost$/UnlessPayer$/UnlessSwitched$ on
// Sacrifice (vexdev). Three shapes exist in the compiled corpus (155 raw
// UnlessCost$ lines over 154 files):
//
//   - the damage-payment offer, UnlessCost$ DamageYou<N> — exactly two cards,
//     Vexing Devil (N=4) and Longhorn Firebeast (N=5), both UnlessPayer$
//     Opponent + UnlessSwitched$ True. Each alive opponent is offered, in
//     turn order starting after the controller (CR 608.2d's one
//     opportunity each), the choice to take N damage; the first acceptance
//     deals it (rules' resume arm emits the Damage event — payment events
//     belong to rules) and the sacrifice proceeds; every decline leaves the
//     permanent in play.
//   - plain mana UnlessCost$ ("1", "B", "G G", "1 U" — the echo /
//     cumulative-upkeep family, UnlessPayer$ You), unswitched: the UnlessCost$
//     resolves through rules' shared unless_pay resume arm, which pays it
//     with payMana. A paid answer spares the permanent; a decline (or an
//     affordable-looking answer the pool cannot cover) sacrifices.
//   - everything else — Sac<>, Discard<>, Return<>, PayLife, PayEnergy,
//     tapXType, ExileFromGrave, SubCounter, RemoveAnyCounter, UpkeepX,
//     DefinedCost_*, and every exotic UnlessPayer$ selector — is
//     deliberately NOT implemented: the gate returns false and today's
//     behaviour stands (an unconditional first-pass sacrifice, decline
//     semantics), so the blast radius stays inside the two shapes above.
//
// fx42 scoping: the answer is taken into a local and Ctx.UnlessPay cleared
// BEFORE anything reads it, so a nested unless-pay consumer reached below
// this gate in the same walk poses its own ask instead of inheriting the
// answer. Sacrifice is a new top-of-walk UnlessPay consumer; the only other
// readers are effCounter and effCopySpellAbility, neither of which reads it
// again after its own top.
func sacrificeUnlessPay(h Host, c *Ctx, sa *cards.SA) bool {
	cost := strings.TrimSpace(sa.Params["UnlessCost"])
	if cost == "" {
		return false
	}
	switched := strings.EqualFold(strings.TrimSpace(sa.Params["UnlessSwitched"]), "True")
	ans := c.UnlessPay
	ansTarget := c.UnlessPayTarget
	c.UnlessPay, c.UnlessPayTarget = "", 0

	if n, dmg := ParseDamageUnlessCost(cost); dmg {
		return sacrificeUnlessDamage(h, c, sa, n, switched, ans, ansTarget)
	}
	if switched {
		// No corpus Sacrifice line carries a switched PLAIN-MANA cost (the
		// four switched lines are the two DamageYou offers and two Sac<>
		// forms, both unimplemented), so an inverted mana ask would be posed
		// with no population to verify it against. Keep today's behaviour.
		return false
	}
	if !isPlainManaCost(cost) {
		// Unpriceable non-mana, non-damage spelling: today's behaviour
		// (unconditional first-pass sacrifice — decline semantics). The
		// rules-side resume arm would decline an unpriceable cost anyway;
		// not asking at all keeps every one of those games byte-identical
		// to the pre-gate engine instead of adding an ask nobody could pay.
		return false
	}
	payer, ok := unlessPayer(h, c, sa)
	if !ok {
		// An exotic UnlessPayer$ selector (Remembered, Player.IsRemembered,
		// TriggeredActivator, ...) cannot be resolved to a player here, and
		// asking the WRONG player is worse than not asking. Today's
		// behaviour (unconditional sacrifice) stands.
		return false
	}
	switch ans {
	case "pay":
		// Re-entry, paid via rules' payMana: the permanent is spared — the
		// body below must not run.
		return true
	case "decline":
		// Re-entry, declined (or the pool could not cover it): the
		// sacrifice proceeds in the body below.
		return false
	}
	shown := unlessCostLabel(cost)
	d := &decision.Decision{Player: payer, Kind: decision.KModes,
		Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay", ResumeSA: sa,
		Prompt: unlessSacrificePrompt(h, c, "Pay "+shown+" to keep it, or sacrifice it"),
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: "Pay " + shown, Obj: c.Source, Player: payer},
			{Index: 1, Kind: "mode", Label: "Sacrifice it", Obj: c.Source, Player: payer},
		}}
	if Ask(h, d) == AskAsked {
		return true // resolution suspended; the answer re-enters this effect.
	}
	// Fuzz/no-engine host: the deterministic decline (R-9) — today's
	// behaviour, the unconditional sacrifice. The Note records why the
	// richer path did not run.
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
		Text: "may pay declined (UnlessCost not asked on this host)"})
	return false
}

// sacrificeUnlessDamage implements the DamageYou<N> offer shape. switched
// (the only corpus population: Vexing Devil, Longhorn Firebeast) asks each
// alive opponent in turn order; the first acceptance — signalled by rules'
// resume arm as UnlessPay "pay" with the Damage event already emitted —
// sacrifices the permanent; every decline leaves it in play. An unswitched
// damage shape has no corpus population today; it is implemented for the
// flag's boolean honesty as the echo orientation (paying the damage SPARES
// the permanent) with the UnlessPayer$-resolved payer, and an unresolvable
// payer keeps today's behaviour.
func sacrificeUnlessDamage(h Host, c *Ctx, sa *cards.SA, n int, switched bool, ans string, ansTarget int) bool {
	if !switched {
		payer, ok := unlessPayer(h, c, sa)
		if !ok {
			return false // today's behaviour: unconditional sacrifice
		}
		switch ans {
		case "pay":
			// rules emitted the payer's damage on the resume arm; spared —
			// the body below must not run.
			return true
		case "decline":
			return false // the sacrifice proceeds below
		}
		d := &decision.Decision{Player: payer, Kind: decision.KModes,
			Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay", ResumeSA: sa,
			Prompt: unlessSacrificePrompt(h, c, "take "+strconv.Itoa(n)+" damage to spare it, or sacrifice it"),
			Options: []decision.Option{
				{Index: 0, Kind: "mode", Label: "Take " + strconv.Itoa(n) + " damage", Obj: c.Source, Player: payer},
				{Index: 1, Kind: "mode", Label: "Sacrifice it", Obj: c.Source, Player: payer},
			}}
		if Ask(h, d) == AskAsked {
			return true
		}
		// No-ask host: the deterministic decline — today's behaviour, the
		// unconditional sacrifice, no damage dealt.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "may pay declined (UnlessCost not asked on this host)"})
		return false
	}
	// Switched: the offer is open to EVERY opponent, one at a time in turn
	// order. On a decline re-entry the cursor (UnlessPayTarget, carried
	// through the resume point) says which opponent declined; the next one
	// is offered, and once the list is exhausted the permanent stays.
	opp := opponentsInTurnOrder(h.Game(), c.Controller)
	if ans == "pay" {
		// The accepting opponent's Damage event was emitted by rules'
		// resume arm; the sacrifice proceeds in the body below.
		return false
	}
	if ans == "decline" {
		if ansTarget+1 >= len(opp) {
			return true // every opponent declined: the permanent stays
		}
		return poseSacrificeDamageOffer(h, c, sa, opp[ansTarget+1], ansTarget+1, n)
	}
	if len(opp) == 0 {
		// No opponent may accept (nobody else alive): the offer is empty.
		return true
	}
	if poseSacrificeDamageOffer(h, c, sa, opp[0], 0, n) {
		return true // resolution suspended; the answer re-enters this effect.
	}
	// Fuzz/no-engine host: the deterministic stand-in takes option 0 (R-9,
	// the accept arm) — the first opponent in turn order takes the damage
	// and the sacrifice proceeds. The damage is emitted here, with the
	// ordinary DealDamage emitter, because with no engine host there is no
	// rules-side resume arm to pay it; in a real engine this branch is
	// unreachable (Engine.Ask always returns true).
	rider := newDamageRider(h, c, sa, int32(n))
	prev := h.SetDamageSource(rider.source)
	emitPlayerDamage(rider, opp[0])
	h.SetDamageSource(prev)
	return false
}

// poseSacrificeDamageOffer offers one opponent the Vexing Devil deal: take N
// damage (and the permanent is sacrificed), or refuse (it stays). Returns
// true when the ask was posed and the resolution suspended. ResumeTarget
// carries the opponent's index in the deterministic turn-order list so the
// decline cursor can continue after exactly that opponent.
func poseSacrificeDamageOffer(h Host, c *Ctx, sa *cards.SA, p state.PlayerID, idx, n int) bool {
	name := "the permanent"
	if o := h.Game().Obj(c.Source); o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	d := &decision.Decision{Player: p, Kind: decision.KModes,
		Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay", ResumeSA: sa,
		ResumeTarget: idx,
		Prompt:       name + " deals " + strconv.Itoa(n) + " damage to you — accept?",
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: "Take " + strconv.Itoa(n) + " damage", Obj: c.Source, Player: p},
			{Index: 1, Kind: "mode", Label: "Refuse — it stays", Obj: c.Source, Player: p},
		}}
	return Ask(h, d) == AskAsked
}

// ParseDamageUnlessCost reports whether an UnlessCost$ value is the
// damage-payment offer form "DamageYou<N>" and returns N. Recognised: the
// exact spelling DamageYou< followed by a positive integer literal and '>'.
// Everything else — a bare SVar name, an X, another primitive's bracket
// spellings — is not, so a future SVar-driven shape fails closed to the
// unimplemented behaviour rather than asking the wrong offer.
func ParseDamageUnlessCost(cost string) (int, bool) {
	s := strings.TrimSpace(cost)
	const head = "DamageYou<"
	if !strings.HasPrefix(s, head) || !strings.HasSuffix(s, ">") {
		return 0, false
	}
	n, err := strconv.Atoi(s[len(head) : len(s)-1])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// unlessPayer resolves Sacrifice's UnlessPayer$ to a player. Recognised:
// the empty default and "You" (the controller — the whole echo /
// cumulative-upkeep population), and "Opponent" (the first alive opponent
// in turn order). Any other selector fails closed: the caller keeps
// today's behaviour instead of asking the wrong player.
func unlessPayer(h Host, c *Ctx, sa *cards.SA) (state.PlayerID, bool) {
	switch strings.TrimSpace(sa.Params["UnlessPayer"]) {
	case "", "You":
		return c.Controller, true
	case "Opponent":
		if opp := opponentsInTurnOrder(h.Game(), c.Controller); len(opp) > 0 {
			return opp[0], true
		}
	}
	return 0, false
}

// opponentsInTurnOrder lists the alive seats other than `you`, in turn
// order starting after `you` (CR 608.2d's one opportunity each, in turn
// order). There is no team model in this build — an opponent is any other
// surviving seat.
func opponentsInTurnOrder(g *state.Game, you state.PlayerID) []state.PlayerID {
	out := make([]state.PlayerID, 0, len(g.Players))
	for _, p := range g.AliveFrom(you) {
		if p != you {
			out = append(out, p)
		}
	}
	return out
}

// unlessSacrificePrompt renders the ask prompt with the offering card's
// name, so a seat reads "Vexing Devil — take 4 damage to spare it, or
// sacrifice it" rather than raw Forge script.
func unlessSacrificePrompt(h Host, c *Ctx, action string) string {
	name := "The permanent"
	if o := h.Game().Obj(c.Source); o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	return name + " — " + action
}
