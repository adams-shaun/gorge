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
	Register("Draw", effDraw)
	Register("Discard", effDiscard)
	Register("Mill", effMill)
	Register("Dig", effDig)
	Register("Reveal", effReveal)
	Register("RevealHand", effReveal)
	Register("PeekAndReveal", effReveal)
	Register("RearrangeTopOfLibrary", effRearrangeTopOfLibrary)
	Register("Scry", effScry)
	Register("Surveil", effSurveil)
	Register("DigUntil", effDigUntil)
	Register("NameCard", effNameCard)
	Register("Hideaway", effHideaway)
}

// zoneOf is a bounds-checked g.Zone. PlayerOf returns a target's raw Player
// field (or, for effNameCard, Ctx.Controller is passed straight through)
// with no validation of its own, and Game.Zone computes
// int(p)*numZones+int(z) and indexes a fixed-size slice without checking
// that p names a real seat -- an out-of-range PlayerID reaches that
// arithmetic and panics with index out of range (Ruling T18-a). Every call
// site in this file that reads a zone to decide what to move goes through
// this rather than g.Zone directly, mirroring count.go's own PlayerID bounds
// check ahead of g.Players[c.Controller]. An invalid seat degrades to nil --
// the same shape as a real, empty zone -- so the primitive simply finds
// nothing there rather than panicking or erroring.
func zoneOf(g *state.Game, z state.Zone, p state.PlayerID) []state.ObjID {
	if int(p) >= len(g.Players) {
		return nil
	}
	return g.Zone(z, p)
}

// DrawFor is exported so the rules package can use the same code path for the
// draw step. Drawing from an empty library is a loss, checked by SBAs.
func DrawFor(h Host, p state.PlayerID) { drawFor(h, p, -1, nil, drawUptoRider{}) }

// drawUptoRider is the Upto$ Draw continuation an in-flight upto batch
// carries across a Dredge ask (Arcane Denial's "may draw up to two" whose
// draw parks on a Dredge replacement): idx is the Defined$ target index
// whose batch is in flight (-1 = no upto in flight, the zero value every
// non-upto caller passes) and count the answered count for it. It rides the
// ask as Decision.ResumeUptoIdx/ResumeUptoCount; rules' dredge arm restores
// the Ctx fields from them.
type drawUptoRider struct {
	idx   int
	count int32
}

// drawFor is DrawFor with an optional enclosing Draw cursor. A nonnegative
// cursor is recorded on a dredge decision so rules can continue that exact
// multi-card resolution after its replacement is answered.
func drawFor(h Host, p state.PlayerID, cursor int, resumeSA *cards.SA, upto drawUptoRider) {
	g := h.Game()
	lib := zoneOf(g, state.ZLibrary, p)
	if len(lib) == 0 {
		h.Emit(events.Event{Kind: events.PlayerLost, Player: p, Text: "drew from an empty library"})
		return
	}
	// A DrawFor reached while the resolution is already suspended: a caller
	// that does not check h.Suspended() between draws drove a second draw
	// after the first one parked on a dredge ask. Posing a second ask here
	// would overwrite the outstanding one (the orphaned-decision failure
	// findings-sol4 proved); the guarded callers (effDraw's cursor loop,
	// the turn draw, rules' lifeReplacementDraw park) never reach this
	// suspended, so this degrades the unguarded one deterministically: no
	// ask, an ordinary draw, one Note naming why.
	if h.Suspended() {
		h.Emit(events.Event{Kind: events.Note, Player: p,
			Text: "drew without a dredge choice: another decision is already pending"})
		h.Emit(events.Event{Kind: events.Draw, Player: p, Obj: lib[0],
			From: state.ZLibrary, To: state.ZHand, Secret: true})
		return
	}
	// Dredge (CR 702.55): before a player draws a card, if they have a card
	// with Dredge in the graveyard they may instead mill N cards (N = the
	// dredge number) and return that card from the graveyard to their hand,
	// and the draw is replaced. This is a player choice at the point of the
	// draw, so it is posed as a mid-resolution KModes choice over every legal
	// dredger in graveyard scan order plus the ordinary draw. A fluff/no-host
	// host declines and draws normally.
	if candidates := dredgeCandidates(g, p); len(candidates) > 0 {
		// Pose every legal replacement plus the ordinary draw. A player with
		// several dredgers chooses which replacement applies (CR 616.1).
		// The upto rider travels so the dredge resume restores the answered
		// up-to batch instead of re-asking its decision.
		riderIdx, riderCount := -1, int32(0)
		if upto.idx >= 0 {
			riderIdx, riderCount = upto.idx, upto.count
		}
		d := &decision.Decision{Player: p, Kind: decision.KModes, Min: 1, Max: 1,
			ResumeKind: "dredge", ResumeTarget: cursor, ResumeSA: resumeSA,
			ResumeUptoIdx: riderIdx, ResumeUptoCount: riderCount,
			Prompt: "Replace draw with Dredge?"}
		for _, candidate := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "dredge",
				Label: "Dredge " + strconv.Itoa(int(candidate.n)) + " (mill, then return " + objName(g, candidate.id) + " to hand)", Obj: candidate.id, Player: p})
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "draw", Label: "Draw card", Player: p})
		if h.Ask(d) {
			return
		}
	}
	h.Emit(events.Event{Kind: events.Draw, Player: p, Obj: lib[0],
		From: state.ZLibrary, To: state.ZHand, Secret: true})
}

type dredgeCandidate struct {
	id state.ObjID
	n  int32
}

// dredgeCandidates returns every graveyard Dredge replacement that is legal:
// CR 702.55 requires enough cards to mill the full number. Graveyard scan
// order is deterministic and becomes the decision's stable option order.
func dredgeCandidates(g *state.Game, p state.PlayerID) []dredgeCandidate {
	library := g.Zone(state.ZLibrary, p)
	var out []dredgeCandidate
	for _, id := range g.Zone(state.ZGraveyard, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		if n, ok := o.Face().KeywordParam("Dredge"); ok {
			if v, err := strconv.Atoi(strings.TrimSpace(n)); err == nil && v > 0 && len(library) >= v {
				out = append(out, dredgeCandidate{id: id, n: int32(v)})
			}
		}
	}
	return out
}

func objName(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return "it"
}

func effDraw(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	if n <= 0 {
		return
	}
	// RememberDrawn$ records every card actually drawn into the resolution's
	// Remembered (Breathstealer's Crypt's reveal-and-maybe-discard chain acts
	// on exactly the drawn card; a library that ran out mid-draw records only
	// what moved). Unread before this — the whole sub-chain saw nothing. A
	// draw that parked on a dredge ask has not happened yet, so the record
	// waits until the draw is real (the suspend check below).
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberDrawn"]), "True")
	targets := actingPlayers(h, c, sa)
	total := int32(len(targets)) * n
	// OptionalDecider$ (Mystic Remora, Rhystic Study — Forge's DrawEffect
	// resolve: optional = hasParam("OptionalDecider")... the decider confirms
	// "do you want to draw N cards?" and a decline skips): the DRAW itself is
	// optional, decided by the NAMED decider, resolved through the same
	// player selector grammar every other resolver owns ("You" — the
	// corpus's dominant value, and its bare "True" spelling — is the
	// resolving controller, the enchantment's controller asking themselves
	// whether to draw off their own trigger; TargetedController is the
	// targeted spell's controller (Vex's "that spell's controller may draw
	// a card"), TriggeredCardController the entering creature's (Selvala),
	// Opponent the controller's opponents). The ask is the same mid-
	// resolution KChoose yes/no every other asking primitive poses, answered
	// through rules' "draw_optional" resume arm into Ctx.DrawOpt; a host that
	// cannot ask keeps the pre-ask mandatory draw (the R-9 degradation). A
	// spec the grammar cannot resolve fails closed below this read's own
	// convention: the pre-ask mandatory draw stays and one loud Note names
	// the unmodelled value. A target with an empty library makes the draw a
	// non-choice — Forge's canDrawAmount guard skips those silently, so the
	// ask only fires when SOME target could actually draw; with none, no
	// question is posed and nothing is drawn (an empty-library draw event is
	// a no-op either way).
	if decider := strings.TrimSpace(sa.Params["OptionalDecider"]); decider != "" && total > 0 {
		answered := c.DrawOpt
		c.DrawOpt = "" // fx42 scoping: consumed once; a nested optional draw poses its own ask
		if answered == "" {
			canDraw := false
			for _, t := range targets {
				if len(zoneOf(h.Game(), state.ZLibrary, PlayerOf(h, c, t))) > 0 {
					canDraw = true
					break
				}
			}
			if canDraw {
				seat := c.Controller
				spec := decider
				if strings.EqualFold(spec, "True") {
					spec = "You"
				}
				askable := true
				if !strings.EqualFold(spec, "You") {
					resolved := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})
					if len(resolved) == 0 || !resolved[0].IsPlayer {
						// The decider's identity is unresolvable — a spec the
						// selector grammar does not carry resolves to no player
						// (the self-default fallback returns an OBJECT, which
						// the IsPlayer gate rejects). Fail closed: the pre-ask
						// mandatory draw stays, one loud Note names the value,
						// and NO ask is posed (a yes/no ask to the controller
						// would be exactly the wrong-seat decision this read
						// exists to avoid).
						h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
							Text: "unmodelled Draw OptionalDecider$ " + decider})
						askable = false
					} else {
						seat = resolved[0].Player
					}
				}
				if askable {
					d := &decision.Decision{Player: seat, Kind: decision.KChoose, Min: 1, Max: 1,
						ResumeKind: "draw_optional", ResumeSA: sa, Source: c.Source,
						Prompt: "Draw " + strconv.Itoa(int(n)) + " card(s)?"}
					d.Options = []decision.Option{
						{Index: 0, Kind: "yes", Label: "Yes — draw", Player: seat},
						{Index: 1, Kind: "no", Label: "No", Player: seat},
					}
					if Ask(h, d) == AskAsked {
						return
					}
				}
			} else {
				answered = "no"
			}
		}
		if answered == "no" {
			// Declined (or no drawable pool): no draw; the walk continues to
			// any SubAbility$ chain.
			return
		}
	}
	// Upto$ True (task mordorparams1: Arcane Denial's "Its controller may
	// draw up to two cards at the beginning of the next turn's upkeep",
	// Truce's "Each player may draw up to two cards"): the draw is a real
	// per-target COUNT choice, one KChoose per Defined$ target before that
	// target's draws, Min 0, Max min(NumCards, the target's library size),
	// options the top cards of the TARGET's own library (the library-search
	// ask's private card options — a decision is visible only to
	// Decision.Player, so no leak). Answered through rules' "draw_upto"
	// resume arm into Ctx.DrawUptoIdx/Count/Answered (the DrawOpt pattern,
	// fx42-scoped per target); a batch that parks on a Dredge choice
	// re-enters through the dredge arm, which restores the in-flight
	// target's cursor from the ask's ResumeUpto rider. A host that cannot
	// ask keeps the pre-ask mandatory draw (the R-9 degradation every
	// effDraw arm takes). A library with fewer than n cards caps the ask at
	// what is there; an empty library is a no-op (never a decision whose
	// only answer is empty — OnlyEmptyAnswer refuses it — and never a
	// mandatory draw event that would mill a player the card only offered
	// to draw).
	if strings.EqualFold(strings.TrimSpace(sa.Params["Upto"]), "True") {
		for idx := int(c.DrawUptoIdx); idx < len(targets); idx++ {
			p := PlayerOf(h, c, targets[idx])
			if !c.DrawUptoAnswered {
				lib := zoneOf(h.Game(), state.ZLibrary, p)
				m := n
				if int32(len(lib)) < m {
					m = int32(len(lib))
				}
				if m <= 0 {
					c.DrawUptoIdx = int32(idx + 1)
					continue
				}
				d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 0, Max: int(m),
					ResumeKind: "draw_upto", ResumeSA: sa, ResumeTarget: idx, Source: c.Source,
					// The walk's Remembered rides the ask (the hidden-library
					// search's ResumeRemembered precedent): the re-entered
					// effDraw recomputes `targets` from Defined$, and for the
					// Remembered-valued selectors -- Arcane Denial's
					// `Defined$ DelayTriggerRemembered` is the corpus shape --
					// a resume that rebuilt an empty set would resolve a
					// DIFFERENT target list than the one the cursor indexes,
					// so the answered count would be drawn for the wrong
					// player or for nobody. The ability-object resume path
					// restores the same set from o.Remembered; this covers
					// every other frame.
					ResumeRemembered: append([]state.Target(nil), c.Remembered...),
					Prompt:           "Draw up to " + strconv.Itoa(int(n)) + " card(s)?"}
				for i := int32(0); i < m; i++ {
					id := lib[i]
					label := "a card"
					if o := h.Game().Obj(id); o != nil && o.Face() != nil {
						label = o.Face().Name
					}
					d.Options = append(d.Options, decision.Option{Index: len(d.Options),
						Kind: "card", Label: label, Obj: id, Player: p})
				}
				if Ask(h, d) == AskAsked {
					return
				}
				// No-host (R-9): the pre-ask mandatory draw of what was offered.
				c.DrawUptoCount, c.DrawUptoAnswered = m, true
			}
			for c.DrawDone < c.DrawUptoCount {
				var lib []state.ObjID
				if remember {
					lib = zoneOf(h.Game(), state.ZLibrary, p)
				}
				drawFor(h, p, int(c.DrawDone), sa, drawUptoRider{idx: idx, count: c.DrawUptoCount})
				if h.Suspended() {
					// A Dredge choice is between individual draws. Its resume
					// point restores this target's cursor (idx, answered count
					// and DrawDone); do not run later targets yet.
					return
				}
				if remember && len(lib) > 0 {
					c.Remembered = append(c.Remembered, state.Target{Obj: lib[0]})
				}
				c.DrawDone++
			}
			c.DrawDone = 0
			c.DrawUptoIdx = int32(idx + 1)
			c.DrawUptoCount, c.DrawUptoAnswered = 0, false
		}
		return
	}
	for c.DrawDone < total {
		p := PlayerOf(h, c, targets[c.DrawDone/n])
		var lib []state.ObjID
		if remember {
			lib = zoneOf(h.Game(), state.ZLibrary, p)
		}
		drawFor(h, p, int(c.DrawDone), sa, drawUptoRider{})
		if h.Suspended() {
			// A Dredge choice is between individual draws. Its resume point
			// carries this cursor; do not run later draws, Remembered or
			// SubAbility$ yet.
			return
		}
		if remember && len(lib) > 0 {
			c.Remembered = append(c.Remembered, state.Target{Obj: lib[0]})
		}
		c.DrawDone++
	}
	// DrawDone is scoped to this primitive like the other Ctx answer fields.
	c.DrawDone = 0
}

// effDiscard moves cards from a player's hand to their graveyard. Which
// cards, and who chooses, is driven by the corpus params that the real
// discard spells use and that this primitive now reads:
//
//   - Mode$ RevealYouChoose (Thoughtseize, Duress): the CASTER — c.Controller,
//     not the discarding player — looks at the target's hand and chooses which
//     card is discarded. This is a real mid-resolution ask: the effect poses a
//     KModes decision over the DiscardValid$-filtered hand, suspends, and on
//     re-entry discards exactly the card Ctx.Discard names (the continuation
//     arm in rules/resolution.go set it from the recorded answer). A host
//     that cannot ask falls back to the deterministic front-card stand-in.
//   - Mode$ TgtChoose (Mind Rot, Faithless Looting, Thirst for Knowledge,
//     Riddlesmith): the ordinary "discard N cards" — the DISCARDING player,
//     p (the target, not the caster), chooses which of their own cards to
//     drop, without the hand being revealed first. Same mid-resolution ask
//     shape as RevealYouChoose, same ResumeKind "discard", but the decision's
//     Player is p and the no-ask fallback takes the front of the FILTERED
//     hand. A hand with fewer eligible cards than NumCards$ discards what it
//     owns and asks nothing (there is no choice to be made), and a hand
//     whose eligible count is at or below NumCards$ likewise resolves
//     deterministically with no question.
//   - Mode$ RevealDiscardAll (Cabal Therapy): a FILTER, not a choice. Every
//     card in the target's hand matching DiscardValid$ is discarded, no ask.
//   - Mode$ Hand (Reforge the Soul, Windfall, Magus of the Wheel, Dark
//     Deal): the whole-hand wheel — every card in the target's hand, in hand
//     order, one events.Discard per card, no ask and no Note (a mandatory
//     line has no choice to record). Forge's DiscardEffect HAND mode
//     discards the ENTIRE hand and never reads NumCards$ there. The
//     Optional$ True variant keeps its deterministic stand-in (whole hand +
//     one Note recording why); a real may-discard election is M4 follow-up
//     work.
//   - Mode$ absent / Random / Defined / LookYouChoose / YouChoose /
//     RevealTgtChoose (the cleanup step, Delve-style costs): still the
//     deterministic front-of-hand discard NumCards times —
//     right, because those paths have no player choice to make (or the
//     approximation is elsewhere), and must not become a question.
//
// DiscardValid$ is a Forge filter spec ("Card.nonLand", "Card.NamedCard"),
// evaluated with MatchesSpecFrom (the same resolver effDig uses for
// ChangeValid$). Its default is "Card". The chooser/target split is what keeps
// Thoughtseize from letting the opponent pick their own discard, and what
// keeps a Mind Rot target's own choice from being made by the caster.
// discardRiders carries the two Discard riders Forge applies per discarded
// card (DiscardEffect -> Player.discard): RememberDiscarded$ records the card
// in the resolution's Remembered set -- the ctx-level set a chained
// SubAbility reads for "the cards discarded this way" -- AND event-backed on
// the source (Forge adds to the host card's remembered list, which persists
// past the resolution and is what Card.IsRemembered matches later);
// RememberDiscardingPlayers$ records the discarding player (Forge's
// discardedMap.keySet(), one corpus line).
type discardRiders struct {
	rememberCards   bool
	rememberPlayers bool
}

func discardRidersOf(sa *cards.SA) discardRiders {
	return discardRiders{
		rememberCards:   strings.EqualFold(sa.Params["RememberDiscarded"], "True"),
		rememberPlayers: strings.EqualFold(sa.Params["RememberDiscardingPlayers"], "True"),
	}
}

// actingPlayers resolves a PLAYER-acting SA's targets: the explicit
// Defined$/targeted set when the script names one; otherwise Forge's
// player-side default (SpellAbilityEffect.getPlayers reads
// paramOrDefault("Defined", "You")) -- the resolving ability's OWN controller,
// not the source object's CURRENT controller. The two agreed everywhere until
// the mid-resolution control-change primitives (RememberControlled$) went
// live: they diverge the moment a resolution steals its own source (Kain,
// Traitorous Dragoon -- "that player gains control of Kain. If they do, you
// draw that many cards" must draw for the original controller, not the
// taker). Object-acting effects keep Defined()'s source-object default. The
// corpus's no-Defined population for these primitives (Draw, Discard, Mill,
// Scry/Surveil, RearrangeTopOfLibrary) is behaviour-identical under the
// change (c.Controller == the source's controller unless a mid-resolution
// control change moved it), so no golden game moves.
func actingPlayers(h Host, c *Ctx, sa *cards.SA) []state.Target {
	if strings.TrimSpace(sa.Params["Defined"]) != "" {
		return Defined(h, c, sa)
	}
	if _, targeted := sa.Params["ValidTgts"]; targeted {
		return Defined(h, c, sa)
	}
	return []state.Target{{Player: c.Controller, IsPlayer: true}}
}

// discardAndRemember emits one discard with the riders bound above.
func discardAndRemember(h Host, c *Ctx, r discardRiders, id state.ObjID, p state.PlayerID) {
	h.Emit(events.Discard(id, p))
	if r.rememberCards {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
		eventRemember(h, c, id)
	}
	if r.rememberPlayers && !targetIn(c.Remembered, state.Target{Player: p, IsPlayer: true}) {
		c.Remembered = append(c.Remembered, state.Target{Player: p, IsPlayer: true})
	}
}

// discardBounds is the min/max Forge's DiscardEffect computes for its
// chooseCardsToDiscardFrom call, shared by the asking arms: AnyNumber$ is
// min 0 with no cap short of the eligible hand, Optional$ lowers min to 0,
// and otherwise min == max == NumCards (capped at the eligible count).
// AnyNumber$ is implemented for the two asking modes (RevealYouChoose and
// TgtChoose); the measured corpus population is TgtChoose-only (22 lines,
// every one also Optional$ True), but the bounds themselves are one Forge
// code path for all modes.
func discardBounds(h Host, c *Ctx, sa *cards.SA, eligible int) (int, int) {
	if strings.EqualFold(sa.Params["AnyNumber"], "True") {
		return 0, eligible
	}
	n := int(Num(h, c, sa, "NumCards", 1))
	if n < 1 {
		n = 1
	}
	if n > eligible {
		n = eligible
	}
	min := n
	if strings.EqualFold(sa.Params["Optional"], "True") {
		min = 0
	}
	return min, n
}

// unlessTypeEligible returns the hand cards matching any comma-separated
// UnlessType$ spec, in hand order -- the unless alternative's candidate set
// (Thirst for Knowledge's "discard an artifact card").
func unlessTypeEligible(g *state.Game, c *Ctx, hand []state.ObjID, unless string) []state.ObjID {
	var out []state.ObjID
	for _, id := range hand {
		for _, spec := range strings.Split(unless, ",") {
			spec = strings.TrimSpace(spec)
			if spec != "" && MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

func effDiscard(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	riders := discardRidersOf(sa)
	// fx42: capture the answered discard choice into a local and clear
	// c.Discard before the target loop. The answer must stay scoped to the
	// discard primitive that asked: a DISCARD reached below this one in the
	// same walk (this effect's SubAbility$ chain) must pose its own ask
	// instead of inheriting this one's answered cards. Capturing first keeps
	// the load-bearing multi-target behaviour intact — every target of a
	// multi-target discard sees the SAME answered list, which is exactly what
	// the old per-target c.Discard read produced. Ctx.Discard's only reader is
	// this primitive, so clearing here is safe.
	answers := c.Discard
	c.Discard = nil
	mode := sa.Params["Mode"]
	valid := sa.Params["DiscardValid"]
	if valid == "" {
		valid = "Card"
	}
	for _, t := range actingPlayers(h, c, sa) {
		p := PlayerOf(h, c, t)
		hand := zoneOf(g, state.ZHand, p)

		switch mode {
		case "RevealYouChoose":
			// Re-entry: the caster's choice was answered and the continuation
			// set Ctx.Discard to the chosen object(s). Discard exactly those
			// that sit in this target's hand — a single-target spell resolves
			// to one card, and the per-hand filter keeps a stray answer from
			// moving an object that left the hand meanwhile.
			if answers != nil {
				for _, id := range answers {
					if !containsID(hand, id) {
						continue
					}
					discardAndRemember(h, c, riders, id, p)
				}
				continue
			}
			// First pass: narrow the target's hand to the cards DiscardValid$
			// allows, then ask the CASTER which to discard.
			eligible := make([]state.ObjID, 0, len(hand))
			for _, id := range hand {
				if MatchesSpecCtx(g, valid, id, c.SpecContext(c.Controller)) {
					eligible = append(eligible, id)
				}
			}
			if len(eligible) == 0 {
				continue
			}
			askMin, askMax := discardBounds(h, c, sa, len(eligible))
			opts := make([]decision.Option, 0, len(eligible))
			for _, id := range eligible {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
					Label: "Discard " + name, Obj: id, Player: c.Controller})
			}
			d := &decision.Decision{Player: c.Controller, Kind: decision.KModes,
				Min: askMin, Max: askMax, Source: c.Source,
				ResumeKind: "discard", ResumeSA: sa,
				Prompt:  "Choose " + strconv.Itoa(askMin) + ".." + strconv.Itoa(askMax) + " card(s) to discard",
				Options: opts}
			if Ask(h, d) == AskAsked {
				return // resolution suspended; the answer re-enters with Ctx.Discard set.
			}
			// Fuzz/no-engine host: the deterministic front-card stand-in
			// (R-9), with the Note that records why the richer path did not run.
			// AskEmpty never reaches here by construction (len(eligible) == 0
			// continues above and Min is n >= 1), but the shared helper owns the
			// guard either way.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "discards its first card (no engine host to ask)"})
			// Kept exactly as it was (ONE card, front of the RAW hand) apart
			// from the riders, so a no-host run replays as before.
			if len(hand) > 0 {
				discardAndRemember(h, c, riders, hand[0], p)
			}

		case "TgtChoose":
			// Re-entry: the discarding player's choice was answered and the
			// continuation set Ctx.Discard to the chosen object(s). Discard
			// exactly those that sit in this target's hand (a per-hand filter
			// keeps a stray answer from moving an object that left the hand
			// meanwhile).
			if answers != nil {
				for _, id := range answers {
					if !containsID(hand, id) {
						continue
					}
					discardAndRemember(h, c, riders, id, p)
				}
				continue
			}
			// First pass: narrow the target's hand to the cards DiscardValid$
			// allows. This is the discarding player's own hand, so the choice
			// is presented to p.
			eligible := make([]state.ObjID, 0, len(hand))
			for _, id := range hand {
				if MatchesSpecCtx(g, valid, id, c.SpecContext(c.Controller)) {
					eligible = append(eligible, id)
				}
			}
			if len(eligible) == 0 {
				continue
			}
			// UnlessType$ (Thirst for Knowledge's "discard two cards unless
			// you discard an artifact card"): the discarding player may instead
			// discard ONE card of the named type. Forge's DiscardEffect reads
			// the key only in its TgtChoose branch, swapping the ordinary
			// numCards pick for chooseCardsToDiscardUnlessType, so that is the
			// only mode this walk widens. The election is a real choice the
			// moment one unless-eligible card is in hand -- discarding the
			// artifact and discarding the full count are answers nobody else
			// can make, and the strict-supersets no-ask rule below would
			// otherwise silently drop the alternative. The election's answer
			// re-enters through the "discard_unless" resume arm; the unless
			// arm's own multi-candidate pick re-uses the ordinary "discard"
			// arm (Min == Max == 1). fx42 scoping: the election is consumed and
			// cleared before any further ask this walk poses.
			elected := c.UnlessElected
			c.UnlessElected = ""
			unlessSpec := strings.TrimSpace(sa.Params["UnlessType"])
			if elected == "unless" && unlessSpec != "" {
				picks := unlessTypeEligible(g, c, hand, unlessSpec)
				if len(picks) == 1 {
					discardAndRemember(h, c, riders, picks[0], p)
					continue
				}
				if len(picks) > 1 {
					opts := make([]decision.Option, 0, len(picks))
					for _, id := range picks {
						name := "a card"
						if o := g.Obj(id); o != nil && o.Face() != nil {
							name = o.Face().Name
						}
						opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
							Label: "Discard " + name + " instead", Obj: id, Player: p})
					}
					d := &decision.Decision{Player: p, Kind: decision.KModes,
						Min: 1, Max: 1, Source: c.Source,
						ResumeKind: "discard", ResumeSA: sa,
						Prompt:  "Discard one " + unlessSpec + " card instead",
						Options: opts}
					if Ask(h, d) == AskAsked {
						return // resolution suspended; the answer re-enters with Ctx.Discard set.
					}
					h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
						Text: "discards its first " + unlessSpec + " card (no engine host to ask)"})
					discardAndRemember(h, c, riders, picks[0], p)
					continue
				}
				// The unless-eligible card left the hand meanwhile; fall through
				// to the ordinary discard below.
			} else if elected == "" && unlessSpec != "" && len(unlessTypeEligible(g, c, hand, unlessSpec)) > 0 {
				nOrd := Num(h, c, sa, "NumCards", 1)
				d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
					Source: c.Source, ResumeKind: "discard_unless", ResumeSA: sa,
					Prompt: "Discard one " + unlessSpec + " card instead of " + strconv.FormatInt(int64(nOrd), 10) + "?",
					Options: []decision.Option{
						{Index: 0, Kind: "unless", Label: "Yes — discard one " + unlessSpec, Player: p},
						{Index: 1, Kind: "ordinary", Label: "No — discard normally", Player: p},
					}}
				if Ask(h, d) == AskAsked {
					return // resolution suspended; the answer re-enters with Ctx.UnlessElected set.
				}
				// Fuzz/no-engine host: the deterministic stand-in takes the
				// unless alternative's first card -- Forge's AI does the same
				// thing (PlayerControllerAi.chooseCardsToDiscardUnlessType
				// discards the min-CMC card of the type whenever one exists),
				// with the Note that records why the richer path did not run.
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "discards its first " + unlessSpec + " card instead (no engine host to ask)"})
				picks := unlessTypeEligible(g, c, hand, unlessSpec)
				discardAndRemember(h, c, riders, picks[0], p)
				continue
			}
			askMin, askMax := discardBounds(h, c, sa, len(eligible))
			if strings.EqualFold(sa.Params["AnyNumber"], "True") {
				// "discard any number of cards": any eligible count from zero
				// up is a real choice the moment one eligible card exists, so
				// the strict-supersets gate does not apply to it -- a hand with
				// exactly one eligible card can still legitimately answer
				// "discard it" or "discard nothing".
			} else if askMin == askMax && askMin == len(eligible) {
				// Only a real choice when there are STRICTLY more eligible cards
				// than must be discarded. A hand with NumCards$ eligible cards (or
				// fewer) must drop all of them with no question: the player could
				// not answer differently, so emitting a decision nobody can
				// meaningfully resolve would just be noise (R-9 contract).
				for _, id := range eligible {
					discardAndRemember(h, c, riders, id, p)
				}
				continue
			}
			opts := make([]decision.Option, 0, len(eligible))
			for _, id := range eligible {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
					Label: "Discard " + name, Obj: id, Player: p})
			}
			d := &decision.Decision{Player: p, Kind: decision.KModes,
				Min: askMin, Max: askMax, Source: c.Source,
				ResumeKind: "discard", ResumeSA: sa,
				Prompt:  "Choose " + strconv.Itoa(askMin) + ".." + strconv.Itoa(askMax) + " card(s) to discard",
				Options: opts}
			if Ask(h, d) == AskAsked {
				return // resolution suspended; the answer re-enters with Ctx.Discard set.
			}
			// Fuzz/no-engine host: the deterministic front-of-ELIGIBLE-hand
			// stand-in (R-9) for the discarding player, with the Note that
			// records why the richer path did not run. AskEmpty is
			// unreachable here by construction (eligible nonempty and strictly
			// greater than n above, Min == Max == n >= 1), but the shared
			// helper owns the guard either way.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "discards its first card (no engine host to ask)"})
			for i := 0; i < askMax; i++ {
				discardAndRemember(h, c, riders, eligible[i], p)
			}

		case "RevealDiscardAll":
			// A FILTER, not a choice (Cabal Therapy): discard every card in
			// the target's hand that DiscardValid$ allows, no matter what
			// NumCards$ says. No ask.
			for _, id := range hand {
				if MatchesSpecCtx(g, valid, id, c.SpecContext(c.Controller)) {
					discardAndRemember(h, c, riders, id, p)
				}
			}

		case "Hand":
			// Mode$ Hand is the whole-hand wheel (Reforge the Soul, Windfall,
			// Magus of the Wheel, Dark Deal): "each player discards their hand".
			// Forge's DiscardEffect HAND mode discards the ENTIRE hand and
			// never reads NumCards$ there; the pre-fix engine fell through to
			// the default arm and discarded only the front card. A mandatory
			// Hand line has no choice to record, so there is no ask and no
			// Note: every card in hand order, one events.Discard per card via
			// discardAndRemember (which applies the RememberDiscarded$ /
			// RememberDiscardingPlayers$ riders per card, exactly what Windfall's
			// "greatest number discarded" draw reads). The measured corpus
			// population (108 raw lines) carries no NumCards$, DiscardValid$ or
			// AnyNumber$, so none is read here.
			if strings.EqualFold(sa.Params["Optional"], "True") {
				// "each player MAY discard their hand and draw N" (5 corpus
				// lines): a real may-discard election is M4 follow-up work; the
				// deterministic stand-in takes the discard and records why.
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "may discard resolved as discard (no engine host to ask)"})
			}
			for _, id := range hand {
				discardAndRemember(h, c, riders, id, p)
			}

		case "Defined":
			// DefinedCards$ names the cards to discard (Breathstealer's
			// Crypt: "that player discards it" — the Remembered drawn card).
			// Only cards still in this target's hand move; everything else
			// (already gone, or never theirs) is skipped. The old default arm
			// ignored DefinedCards$ entirely and discarded the front of hand.
			if dc := strings.TrimSpace(sa.Params["DefinedCards"]); dc != "" {
				for _, t := range discardDefinedCards(h, c, dc) {
					if t.IsPlayer {
						continue
					}
					for _, id := range hand {
						if id == t.Obj {
							h.Emit(events.Discard(id, p))
							break
						}
					}
				}
				break
			}
			fallthrough

		default:
			// Deterministic discard from the top of hand order. Real discard is
			// a choice, but the clean-up step and Delve-style costs have no
			// player to ask and must stay exactly as they were; "first in hand"
			// is deterministic and adequate there.
			n := Num(h, c, sa, "NumCards", 1)
			for i := int32(0); i < n; i++ {
				// Re-read the hand each iteration (B2): events.remove rebuilds
				// the zone slice rather than mutating it in place, so a hand
				// captured once — as this primitive used to — never sees the
				// card it just moved, and a NumCards$ >= 2 discard emits the
				// SAME front card N times instead of N distinct cards. Reading
				// the zone per iteration is what the original pre-hoist code
				// did, and is what makes the N-card discard honest.
				cur := zoneOf(g, state.ZHand, p)
				if len(cur) == 0 {
					break
				}
				discardAndRemember(h, c, riders, cur[0], p)
			}
		}
	}
}

// containsID reports whether id is present in ids.
func containsID(ids []state.ObjID, id state.ObjID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// effMill moves cards from the top of a player's library straight to their
// graveyard -- Discard's sibling, minus the hand. With RememberMilled$ True
// every card it actually moves joins the resolution's remembered set, the
// same both-halves recording discardAndRemember does, so a chained pickup
// ("put a card from among them into your hand") filtering on IsRemembered
// finds them instead of silently failing to find.
func effMill(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	if n < 0 {
		n = 0
	}
	remember := strings.EqualFold(sa.Params["RememberMilled"], "True")
	g := h.Game()
	for _, t := range actingPlayers(h, c, sa) {
		p := PlayerOf(h, c, t)
		for i := int32(0); i < n; i++ {
			lib := zoneOf(g, state.ZLibrary, p)
			if len(lib) == 0 {
				break
			}
			id := lib[0]
			h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: state.ZGraveyard, Player: p})
			if remember {
				rememberMilled(h, c, id)
			}
		}
	}
}

// rememberMilled records one milled card in both halves of the remembered
// state, exactly as discardAndRemember does for a discarded one: the
// resolution's Ctx.Remembered set (what a chained sub-ability and an in-
// flight hidden pick filter read this walk) and the source object's event-
// backed Remembered list (what survives the resolution for a later
// Card.IsRemembered / Count$RememberedSize read).
func rememberMilled(h Host, c *Ctx, id state.ObjID) {
	c.Remembered = append(c.Remembered, state.Target{Obj: id})
	eventRemember(h, c, id)
}

// effDig implements Forge's Dig: look at the top DigNum cards of Defined$'s
// library, move up to ChangeNum of the ones matching ChangeValid$ (default
// "Card") to DestinationZone$ (default "Hand"), and leave everything else
// exactly where it already is -- on top of the library, in its existing
// relative order (the remainder-ordering decision is a separate, still-open
// ask; see the row's end). The remainder's second destination DOES exist in
// the corpus as "DestinationZone2$" (with "LibraryPosition2$" placing it in
// a library) -- an earlier note here wrongly claimed the parameter does not
// exist; it is read below. LibraryPosition$ (the PRIMARY move's position,
// 96 corpus lines) is still unread.
//
// A real card can also write "ChangeNum$ All" (e.g. Goblin Guide's own Dig)
// to mean every matching card within the DigNum look, with no cap short of
// that. changeNum's own upper bound is already the size of the dug slice, so
// defaulting it to digNum and only overriding that default for a literal or
// SVar ChangeNum$ handles "All" for free: it is simply the case where
// nothing narrows the cap below the number of cards looked at.
//
// The look-and-take ask (dig1): where the top DigNum window holds STRICTLY
// more ChangeValid$-eligible cards than ChangeNum, the pick is a real
// decision and effDig poses it -- the same strict-supersets rule effDiscard's
// TgtChoose arm already uses, so a decision nobody could answer differently
// is never emitted. ChangeNum$ 0 (the corpus's three reveal-machinery digs:
// birthing_ritual, sanity_grinding, stomping_slabs) takes nothing, so the
// ask gate also requires changeNum > 0 -- otherwise a zero cap with any
// eligible card would pose a Min==Max==0 KChoose whose only legal answer is
// the empty one, a decision nobody could answer differently by definition.
// ChangeNum$ Any is the third cap: Forge's any-number take (Jace, the Mind
// Sculptor's "You may put that card on the bottom", Through the Forest
// Gate's "put any number of land cards"), so it caps at the window and
// lowers the ask's Min to 0 whenever an eligible card exists -- a real
// choice, unlike what an earlier note here claimed take-all (measured: the
// unresolvable-value Num read degraded Any to 0 and those digs took
// NOTHING). The look is recorded first as a Secret Note carrying the
// window's ids (only the library's owner may know what sat on top; the same
// channel effRearrangeTopOfLibrary uses, with the ids added so the owner's
// client can render what was seen -- view/redact.go rule (1) passes a Secret
// event's payload to its own Player and strips it from everyone else). The
// decision is a KChoose over the ELIGIBLE cards only, in library order: an
// ineligible card must not be pickable, so it is not offered (the window
// itself is on the look Note; the prompt names the card text). Min honours
// Optional$ -- 0 when the take is optional, ChangeNum when it is not -- and
// Max is ChangeNum. The answer re-enters through ResumeKind "dig" with
// Ctx.Dig/DigDone and the asking target's index set (rules/resolution.go),
// scoped to this primitive like every other Ctx answer field.
//
// A host that cannot answer (the fuzz/no-engine stand-in, R-9) and the
// no-choice path (eligible <= ChangeNum) keep M1's silent behaviour
// deterministically: the first ChangeNum eligible cards in zone order move,
// the rest stay exactly where they are. A resumed multi-target Dig applies
// the answer only to the target that asked, skips earlier targets that already
// completed before suspension, and preserves that same deterministic behaviour
// for every later target; chained per-library asks remain separate work. On the
// no-choice path nothing new is emitted at all, so a game that never reaches a
// strict-superset Dig replays byte-identically to the pre-dig1 engine.
//
// The variant params (task inbox-paramcensus-dig-variants), each read
// below:
//
//   - Reveal$ True reveals the dug window to the whole table BEFORE any
//     take -- a non-Secret Note carrying the window's ids, the same shape
//     effReveal's public arm emits (Ad Nauseam "Reveal the top card", Chaos
//     Warp, Goblin Guide, Matter Reshaper). It replaces the ask path's
//     private look when a real choice follows the reveal.
//   - NoReveal$ True suppresses that window reveal. A guard, not a
//     behaviour change: the corpus's 84 NoReveal lines carry no Reveal$, and
//     the dig's moves were already Secret before this task (Impulse).
//   - ForceRevealToController$ True reveals each MOVED card publicly before
//     its Secret move -- Ancient Stirrings' "you may reveal a colorless card
//     from among them and put it into your hand" (the window itself stays
//     private). Suppressed when the window was already revealed.
//   - Tapped$ True taps a card the primary move sends to the battlefield,
//     the same MoveZone-then-Tap pair effChangeZone's Tapped$ movers emit
//     (Through the Forest Gate: "put any number of land cards ... onto the
//     battlefield tapped").
//   - DestinationZone2$ (with LibraryPosition2$) is the remainder's second
//     destination: every window card the primary move did not take goes
//     there (Chaos Warp and Goblin Guide's unmatched card back to the
//     library, Matter Reshaper's unmatched card to the hand).
//   - LibraryPosition2$ places a library DestinationZone2$: "0" = top,
//     which is exactly the engine's stay-in-place default, so the placement
//     emits nothing; "-1" = bottom, a real library-to-library move (the
//     MoveZone append lands it at the bottom); anything else is named in a
//     loud Note and the card stays (the corpus carries only "0" and "-1").
//   - SkipReorder$ True is the engine's remainder contract itself -- the
//     untaken cards never move, so they stay on top in their existing
//     relative order -- and it also suppresses a DestinationZone2$
//     remainder placement (the corpus never pairs the two; Through the
//     Forest Gate carries it without one).
//
// Still unread here (each a real divergence, named in AGENTS.md's Dig row):
// Optional$ on the NO-CHOICE path (eligible <= ChangeNum still takes all
// eligible; ChangeNum$ 0 takes nothing silently, correctly), the remainder-ordering decision ("the rest on the bottom in any
// order"; RestRandomOrder$), the primary LibraryPosition$ (96 corpus lines
// put the PRIMARY take at a library position),
// Choser$ (the opponent-chooses planeswalker shape) and the exotic
// DestinationZone2 values (PlanarDeck).
func effDig(h Host, c *Ctx, sa *cards.SA) {
	digNum := Num(h, c, sa, "DigNum", 1)
	if digNum < 0 {
		digNum = 0
	}
	changeNum := digNum
	anyNum := false
	if raw := sa.Params["ChangeNum"]; raw != "" && raw != "All" {
		if raw == "Any" {
			// Forge's any-number cap: the take is uncapped within the window
			// and the answer may be empty -- a real choice whenever any
			// eligible card exists, which the ask gate and Min below read.
			anyNum = true
		} else {
			changeNum = Num(h, c, sa, "ChangeNum", digNum)
		}
	}
	if changeNum < 0 {
		changeNum = 0
	}
	// WithTotalCMC$ is a cumulative mana-value budget over the picked cards
	// ("put any number of nonland permanent cards with total mana value 4 or
	// less"): a card whose own mana value exceeds it can never be picked, and
	// the running sum of the picks must not exceed it either. Absent the
	// param (the corpus default) the budget is 0 and every read below is a
	// no-op, so a non-budget Dig emits byte-identically. Present but
	// unresolvable degrades to budget 0 -- Num's documented convention, "the
	// card does nothing" -- and takes nothing.
	budget, hasBudget := NumResolved(h, c, sa, "WithTotalCMC", 0)
	if budget < 0 {
		budget = 0
	}
	spec := sa.Params["ChangeValid"]
	if spec == "" {
		spec = "Card"
	}
	spec = permanentCardSpec(spec)
	destName := sa.Params["DestinationZone"]
	if destName == "" {
		destName = "Hand"
	}
	dest := ParseZone(destName)
	optional := sa.Params["Optional"] == "True"
	// The variant params (see the comment block above the function for what
	// each means and which corpus card carries it).
	revealWin := strings.EqualFold(strings.TrimSpace(sa.Params["Reveal"]), "True") &&
		!strings.EqualFold(strings.TrimSpace(sa.Params["NoReveal"]), "True")
	forceReveal := strings.EqualFold(strings.TrimSpace(sa.Params["ForceRevealToController"]), "True")
	skipReorder := strings.EqualFold(strings.TrimSpace(sa.Params["SkipReorder"]), "True")
	tapped := strings.EqualFold(strings.TrimSpace(sa.Params["Tapped"]), "True")
	dest2Name := strings.TrimSpace(sa.Params["DestinationZone2"])
	pos2 := strings.TrimSpace(sa.Params["LibraryPosition2"])
	// fx42 scoping: capture and clear the answered pick BEFORE the target
	// loop. DigTarget identifies the exact target that asked: earlier targets
	// completed before suspension and must be skipped, that target consumes
	// the answer, and later targets retain M1's deterministic processing until
	// chained per-library asks exist. A nested Dig below this walk therefore
	// poses its own ask instead of inheriting any of these fields.
	digAns := c.Dig
	digDone := c.DigDone
	digTarget := c.DigTarget
	c.Dig, c.DigDone, c.DigTarget = nil, false, 0
	g := h.Game()
	for targetIndex, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := zoneOf(g, state.ZLibrary, p)
		n := digNum
		if int32(len(lib)) < n {
			n = int32(len(lib))
		}
		top := append([]state.ObjID(nil), lib[:n]...)
		// take moves one window card to the primary destination, revealing
		// it first when ForceRevealToController$ asks (a public Note naming
		// the card, then the Secret move -- the same reveal-then-secret-move
		// shape effChangeZone's Reveal$ fetch emits; suppressed when Reveal$
		// already made the whole window public) and tapping it right after a
		// Tapped$ True battlefield entry.
		take := func(id state.ObjID) {
			if forceReveal && !revealWin {
				h.Emit(events.Event{Kind: events.Note, Player: p, IDs: []state.ObjID{id}})
			}
			ev := moveZoneEvent(c, id, state.ZLibrary, dest)
			ev.Player, ev.Secret = p, true
			// ExileFaceDown$ True with an exile destination (Ugin, the
			// Ineffable's [+1]: "Exile the top card of your library face down
			// and look at it") carries the same face-down exile payload
			// Hideaway's move uses -- the exiling source rides in Amount --
			// so a replay derives FaceDown identically and a projection
			// withholds the card.
			if strings.EqualFold(strings.TrimSpace(sa.Params["ExileFaceDown"]), "True") && dest == state.ZExile {
				ev.Counter = "exiled_with_face_down"
				ev.Amount = int32(c.Source)
			}
			h.Emit(ev)
			digRemember(c, sa, id)
			if tapped && dest == state.ZBattlefield {
				h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: p, Text: "entered tapped"})
			}
			// StaticEffect$ on a battlefield take (Arbiter of the Ideal's
			// "put it onto the battlefield ... it's an enchantment"): the same
			// rider registration every ChangeZone mover applies.
			if dest == state.ZBattlefield {
				applyStaticEffect(h, c, sa, dest, []state.ObjID{id})
			}
		}
		// rest moves the window cards the primary move did not take to the
		// second destination (DestinationZone2$, placed by LibraryPosition2$).
		// With no DestinationZone2$ -- and with SkipReorder$ True -- the cards
		// stay exactly where they are, so the no-variant games emit nothing
		// and replay byte-identically.
		rest := func(ids []state.ObjID) {
			if dest2Name == "" || skipReorder {
				return
			}
			dest2 := ParseZone(dest2Name)
			for _, id := range ids {
				if dest2 == state.ZLibrary {
					// Library placement: "0" (top) is the engine's
					// stay-in-place default -- the remaining window cards
					// already sit on top in their existing relative order, so
					// the placement is no event; "-1" (bottom) is a real
					// library-to-library move (Move's zone append lands it at
					// the bottom); anything else is named loudly and the card
					// stays.
					if pos2 == "-1" {
						h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
							From: state.ZLibrary, To: state.ZLibrary, Player: p, Secret: true})
					} else if pos2 != "" && pos2 != "0" {
						h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: p,
							Text: "LibraryPosition2$ " + pos2 + " is not implemented; the card stays on top"})
					}
					continue
				}
				if forceReveal && !revealWin {
					h.Emit(events.Event{Kind: events.Note, Player: p, IDs: []state.ObjID{id}})
				}
				ev := moveZoneEvent(c, id, state.ZLibrary, dest2)
				ev.Player, ev.Secret = p, true
				h.Emit(ev)
				digRemember(c, sa, id)
			}
		}
		if digDone && targetIndex < digTarget {
			// This target completed on the first pass before a later library
			// suspended the effect. Re-running it could move a second batch (or
			// newly create a choice after its first batch left), so skip it.
			continue
		}
		if digDone && targetIndex == digTarget {
			// Re-entry: move exactly the answered cards that still sit in the
			// ASKING target's window (a per-window filter keeps a stray answer
			// from moving an object that left the window meanwhile), in the
			// player's answer order; the rest of the window goes to the
			// second destination.
			picked := make(map[state.ObjID]bool, len(digAns))
			moved := make([]state.ObjID, 0, len(digAns))
			for _, id := range digAns {
				if !containsID(top, id) {
					continue
				}
				picked[id] = true
				take(id)
				moved = append(moved, id)
			}
			restIDs := make([]state.ObjID, 0, len(top))
			for _, id := range top {
				if !picked[id] {
					restIDs = append(restIDs, id)
				}
			}
			rest(restIDs)
			continue
		}
		eligible := make([]state.ObjID, 0, len(top))
		for _, id := range top {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
			}
		}
		// affordable says whether a card may be picked at all under
		// WithTotalCMC$: a card whose own mana value exceeds the budget can
		// never fit, however few are taken.
		affordable := func(id state.ObjID) bool { return !hasBudget || manaValueOf(g, id) <= int(budget) }
		// budgetEligible is the pickable set: spec-matching AND individually
		// affordable (no budget => identical to eligible).
		budgetEligible := eligible
		if hasBudget {
			budgetEligible = make([]state.ObjID, 0, len(eligible))
			for _, id := range eligible {
				if affordable(id) {
					budgetEligible = append(budgetEligible, id)
				}
			}
		}
		// greedy is the deterministic forced take under the cumulative budget:
		// walk budgetEligible in zone order and take each card only while the
		// running sum of mana values still fits. This is the exact take the
		// no-choice tail and the R-9 no-host fallback apply, and it is also
		// what decides whether a choice exists (below).
		greedy := make([]state.ObjID, 0, len(budgetEligible))
		running := 0
		for _, id := range budgetEligible {
			if int32(len(greedy)) >= changeNum {
				break
			}
			mv := manaValueOf(g, id)
			if hasBudget && running+mv > int(budget) {
				continue
			}
			running += mv
			greedy = append(greedy, id)
		}
		// forcedAll says the forced greedy take consumes every budget-eligible
		// card, so the answer cannot differ from it and no ask is warranted.
		forcedAll := len(greedy) == len(budgetEligible)
		askBudget := hasBudget && len(budgetEligible) > 0 && !forcedAll
		if !digDone && changeNum > 0 && ((int32(len(eligible)) > changeNum || anyNum && len(eligible) > 0) || askBudget) {
			// A real choice: record the look, then ask the library's owner.
			// Reveal$ True makes the record a PUBLIC reveal of the window (the
			// same non-Secret ids-Note shape effReveal's public arm emits);
			// otherwise the look stays private to the library's owner.
			if revealWin {
				h.Emit(events.Event{Kind: events.Note, Player: p, IDs: top})
			} else {
				emitLook(h, []state.PlayerID{p}, state.ZLibrary, top, "looks at the top of the library")
			}
			minv := int32(0)
			if !optional && !anyNum {
				minv = changeNum
			}
			// A mandatory budget dig whose changeNum exceeds what the budget
			// affords must not demand more picks than it can pay for: lower the
			// Min to the forced affordable count so the ask can be satisfied.
			// (Corpus carriers are all Min 0; this is general-correctness code.)
			if hasBudget && minv > int32(len(greedy)) {
				minv = int32(len(greedy))
			}
			verb := "you may put up to "
			if !optional && !anyNum {
				verb = "put "
			}
			prompt := "Look at the top " + strconv.Itoa(int(n)) + " card(s) of your library: " + verb + strconv.Itoa(int(changeNum)) + " matching card(s) into " + digDestPhrase(dest)
			if hasBudget {
				prompt += " (total mana value " + strconv.Itoa(int(budget)) + " or less)"
			}
			d := &decision.Decision{Player: p, Kind: decision.KChoose,
				Min:          int(minv),
				Max:          int(changeNum),
				MaxSum:       int(budget),
				Source:       c.Source,
				ResumeKind:   "dig",
				ResumeSA:     sa,
				ResumeTarget: targetIndex,
				Prompt:       prompt}
			for _, id := range budgetEligible {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				opt := decision.Option{Index: len(d.Options),
					Kind: "dig", Label: name, Obj: id, Player: p}
				// Only a budget Dig carries a Value: Option.Value is
				// omitempty, and setting it on a budget-less Dig would put a
				// "value" field on the wire for every offered card although
				// MaxSum is 0 and nothing reads it. Keeping it budget-only
				// leaves every existing (non-budget) option list serialising
				// byte-identically.
				if hasBudget {
					opt.Value = manaValueOf(g, id)
				}
				d.Options = append(d.Options, opt)
			}
			if Ask(h, d) == AskAsked {
				return // resolution suspended; the answer re-enters with Ctx.Dig set.
			}
			// Fuzz/no-engine host: the deterministic stand-in (R-9) takes the
			// greedy affordable set -- the exact mirror of the budget-aware bot
			// arm -- with the Note that records why the richer path did not run.
			// AskEmpty is unreachable here by construction (the ask gate requires
			// options >= 1), but the shared helper owns the guard either way.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: p,
				Text: "takes the first matching card(s) (no engine host to ask)", Secret: true})
			taken := make(map[state.ObjID]bool, len(greedy))
			for _, id := range greedy {
				take(id)
				taken[id] = true
			}
			restIDs := make([]state.ObjID, 0, len(top))
			for _, id := range top {
				if !taken[id] {
					restIDs = append(restIDs, id)
				}
			}
			rest(restIDs)
			continue
		}
		// No choice to ask about: M1's silent behaviour for the no-variant
		// cards -- only a Reveal$ window reveal, a Tapped$ Tap or a
		// DestinationZone2$ remainder move can add an event, and only a card
		// carrying those emits one. The forced greedy take moves in window
		// order while the cumulative budget (WithTotalCMC$) allows; everything
		// else in the window -- unmatched, over-budget and beyond the cap
		// alike -- goes to the second destination or stays exactly where it is.
		if revealWin && len(top) > 0 {
			h.Emit(events.Event{Kind: events.Note, Player: p, IDs: top})
		}
		taken := make(map[state.ObjID]bool, len(greedy))
		for _, id := range greedy {
			take(id)
			taken[id] = true
		}
		restIDs := make([]state.ObjID, 0, len(top))
		for _, id := range top {
			if !taken[id] {
				restIDs = append(restIDs, id)
			}
		}
		rest(restIDs)
	}
}

// manaValueOf is the offered card's own mana value -- the same Face().Cmc()
// read state.SacrificedInfoOf uses. It is the per-card price under a
// WithTotalCMC$ cumulative budget (Decision.MaxSum): every budgeted picker
// (Dig, the hidden pick, the library search, the Play grant) shares this one
// read so the four cannot drift on what a card's mana value is.
func manaValueOf(g *state.Game, id state.ObjID) int {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return int(o.Face().Cmc())
	}
	return 0
}

// digRemember honours a Dig's RememberChanged$ True: each card the dig moved
// joins the resolution's Remembered, where a chained SubAbility$ reads it --
// Atsushi's DBEffect RememberObjects$ RememberedCard seeds the registered
// may-play grant's Remembered from exactly this list. Absent the parameter
// (the corpus default) the walk adds nothing, so every pre-existing game
// replays byte-identically.
func digRemember(c *Ctx, sa *cards.SA, id state.ObjID) {
	if strings.EqualFold(strings.TrimSpace(sa.Params["RememberChanged"]), "True") {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
	}
}

// digDestPhrase names the take's destination in the human-readable prompt;
// it is Dig's own phrasing (the picked card GOES to the destination, unlike
// KArrange's Kind which names pile B's), kept separate from
// destinationPhrase so the two vocabularies cannot drift into each other.
// The library arm says the BOTTOM because that is where the take lands:
// events.Move appends to the destination zone, so a library take is a
// move to the bottom -- which is exactly the shape the corpus's
// library-destination digs describe (Jace, the Mind Sculptor's "you may
// put that card on the bottom", mesmeric_sliver's LibraryPosition$ -1).
// A take at a DIFFERENT library position (the primary LibraryPosition$, e.g.
// munda_ambush_leader's "0") is still unread -- the prompt describes what
// the engine does, not what the card asks.

// permanentCardSpec rewrites a leading `Permanent` base token to
// `PermanentCard` -- the shared matcher's battlefield-object base -- so a
// spec evaluated against cards AWAY from the battlefield reads the base as
// Forge's "permanent card": a Dig's ChangeValid$ window is always the
// library, where a bare Permanent must mean Chaos Warp's "If it's a
// permanent card" and Matter Reshaper's "if it's a permanent card with
// mana value 3 or less", never the on-the-battlefield reading
// matchesBase gives the base. rules/stack.go's targetSpecForZone carries
// the same rewrite for target specs; the two cannot share code (effects
// must not import rules), only the rule, and only the leading token is
// rewritten -- every qualifier rides along.
func permanentCardSpec(spec string) string {
	if spec == "Permanent" {
		return "PermanentCard"
	}
	if len(spec) > len("Permanent") && spec[:len("Permanent")] == "Permanent" {
		switch spec[len("Permanent")] {
		case '.', '+', ',':
			return "PermanentCard" + spec[len("Permanent"):]
		}
	}
	// Forge's OTHER spelling of the same base, `Card.Permanent[.rest|+pred]` (Ao,
	// the Dawn Sky's `ChangeValid$ Card.Permanent+nonLand`): the shared
	// matcher reads a `Permanent` PREDICATE as deliberately fail-closed
	// (see matchesObjectText's contextualSameName comment), because a bare
	// `Permanent` base already means an on-the-battlefield object -- a Dig
	// window is always the library, so `Card.Permanent` there must mean
	// Forge's permanent CARD. Rewrite the leading `Card.Permanent` to the
	// internal `PermanentCard` base, moving the predicate separator to the
	// dot the grammar requires (`Card.Permanent+nonLand` ->
	// `PermanentCard.nonLand`; `Card.Permanent` -> `PermanentCard`).
	if rest, ok := strings.CutPrefix(spec, "Card.Permanent"); ok {
		switch {
		case rest == "":
			return "PermanentCard"
		case rest[0] == '.':
			return "PermanentCard" + rest
		case rest[0] == '+', rest[0] == ',':
			// The leading `Permanent` predicate (or OR alternative) is
			// absorbed into the base: drop the consumed separator and
			// re-join the tail with the dot the predicate grammar needs.
			tail := rest[1:]
			if rest[0] == ',' {
				return "PermanentCard," + tail
			}
			return "PermanentCard." + tail
		}
	}
	return spec
}
func digDestPhrase(dest state.Zone) string {
	switch dest {
	case state.ZHand:
		return "your hand"
	case state.ZGraveyard:
		return "your graveyard"
	case state.ZExile:
		return "exile"
	case state.ZBattlefield:
		return "the battlefield"
	case state.ZLibrary:
		return "the bottom of your library"
	default:
		return "its destination"
	}
}

// effDigUntil implements Forge's reveal-until search (DigUntilEffect; task
// diguntil1) — a DIFFERENT primitive from Dig, with its own param family:
// reveal cards from the top of Defined$'s library (default: the resolving
// controller) in zone order until one matches Valid$, publicly reveal every
// card turned over INCLUDING the found one (the same non-Secret ids-Note
// shape effDig's Reveal$ arm emits), move the found card(s) to
// FoundDestination$ (default Hand) and the revealed rest to
// RevealedDestination$ (default Library; RevealedLibraryPosition$ "-1" =
// bottom via Move's zone append, "0"/absent = the stay-in-place default so
// no event). The cards AFTER the found card stay in the library untouched —
// the scan stops at the amount-th match, unlike Dig's fixed window.
//
// OptionalFoundMove$ True (4 corpus lines — Songbirds' Blessing) makes the
// found move a real yes/no ask to the library's owner (the attach_optional
// KChoose shape): "yes" moves to FoundDestination$, "no" — the decline —
// to OptionalNoDestination$ when the SA carries one, else the found card
// JOINS the revealed pile (the corpus oracles all say "then put all cards
// revealed this way that weren't put onto the battlefield on the bottom":
// Genesis Storm, Hei Bai, Aurora Awakener). A no-host (AskNoHost) declines
// deterministically (R-9); botpolicy's clamp fallback answers option 0 =
// "yes". RevealRandomOrder$ True (54 lines) means the revealed pile would
// return "in a random order" — randomness is forbidden here, so the
// deterministic stand-in returns them in their existing library order
// (recorded in AGENTS.md's Known approximations).
//
// Riders implemented: RememberFound$ / RememberRevealed$ (the ctx-level
// Remembered discipline digRemember uses), Tapped$ (the MoveZone-then-Tap
// pair effDig's battlefield take emits) and GainControl$ (the found
// permanent enters under the resolving controller via the ordinary
// ControlChange event). A found card with an AURA face put onto the
// battlefield gets the CR 303.4f non-cast-entry attach, degraded to the
// deterministic stand-in: the first permanent in the controller's
// battlefield zone order that satisfies the Enchant keyword's own spec (the
// same read rules/attach.go's auraStillMatchesEnchant does; effects keeps
// its own copy — it must not import rules). With NO eligible bearer the
// Aura stays in the library (CR 303.4f's remain-in-current-zone) rather
// than entering unattached and dying to the CR 704.5m SBA. Riders withheld with ONE loud Note naming each and
// the core move still running: Amount$ non-literal (X/MassX/VoteNum/Y —
// amount 1 then; literal 1..5 ARE honoured as "keep revealing until N
// matches"), DigZone$ (only Library is a real zone — the PlanarDeck
// carriers scan no zone at all and move nothing), NoMoveFound$ /
// FoundLibraryPosition$ (the found card stays where it is), Shuffle$ /
// ShuffleCondition$ (the revealed rest go to RevealedDestination$ in
// existing order instead of shuffling in), ImprintFound$ /
// ImprintRevealed$ (no imprint association is recorded).
func effDigUntil(h Host, c *Ctx, sa *cards.SA) {
	spec := sa.Params["Valid"]
	if spec == "" {
		spec = "Card"
	}
	spec = permanentCardSpec(spec)
	foundDestName := sa.Params["FoundDestination"]
	if foundDestName == "" {
		foundDestName = "Hand"
	}
	foundDest := ParseZone(foundDestName)
	revDest := state.ZLibrary
	if raw := strings.TrimSpace(sa.Params["RevealedDestination"]); raw != "" {
		revDest = ParseZone(raw)
	}
	revPos := strings.TrimSpace(sa.Params["RevealedLibraryPosition"])
	optionalMove := strings.EqualFold(strings.TrimSpace(sa.Params["OptionalFoundMove"]), "True")
	noMoveRevealed := strings.EqualFold(strings.TrimSpace(sa.Params["NoMoveRevealed"]), "True")
	tapped := strings.EqualFold(strings.TrimSpace(sa.Params["Tapped"]), "True")
	gainControl := strings.EqualFold(strings.TrimSpace(sa.Params["GainControl"]), "True")
	rememberFound := strings.EqualFold(strings.TrimSpace(sa.Params["RememberFound"]), "True")
	rememberRevealed := strings.EqualFold(strings.TrimSpace(sa.Params["RememberRevealed"]), "True")
	amount := int32(1)
	var withheld []string
	if raw := strings.TrimSpace(sa.Params["Amount"]); raw != "" && raw != "1" {
		if n, err := strconv.Atoi(raw); err == nil && n > 1 && n <= 5 {
			amount = int32(n)
		} else {
			withheld = append(withheld, "Amount$ "+raw)
		}
	}
	// The params that keep the FOUND card where it is: NoMoveFound$ True is
	// the card's own instruction, and the FoundLibraryPosition$ carriers are
	// all position "0" — already on top — so the no-move read is the
	// behaviour the card names, with the loud Note saying the primitive is
	// not the full param.
	noMoveFound := false
	for _, key := range []string{"NoMoveFound", "FoundLibraryPosition"} {
		if v := digUntilParamValue(sa, key); v != "" {
			withheld = append(withheld, key+"$ "+v)
			noMoveFound = true
		}
	}
	// Purely inert riders: one loud Note, the move proceeds without them.
	for _, key := range []string{"Shuffle", "ShuffleCondition", "ImprintFound", "ImprintRevealed"} {
		if v := digUntilParamValue(sa, key); v != "" {
			withheld = append(withheld, key+"$ "+v)
		}
	}
	// fx42 scoping: capture and clear the answered found-move election BEFORE
	// the target loop, so a nested DigUntil in the same chain poses its own
	// ask instead of inheriting the answer. moveDone also suppresses the
	// re-emit of the reveal Note (recorded before the first-pass ask) and of
	// the withheld-params Note.
	moveAns := c.DigUntilMove
	moveDone := c.DigUntilMoveDone
	c.DigUntilMove, c.DigUntilMoveDone = "", false
	if moveAns == "" {
		moveAns = "no"
	}
	g := h.Game()
	if !moveDone && len(withheld) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "DigUntil withholds " + strings.Join(withheld, ", ") + "; the core move runs without it"})
	}
	targets := Defined(h, c, sa)
	if sa.Params["Defined"] == "" && sa.Params["ValidTgts"] == "" {
		// Forge's default for a reveal-until with no Defined$ and no targets:
		// the resolving controller's own library (Songbirds' Blessing's
		// trigger). Defined's source-object fallback is wrong here — the
		// source is the resolving permanent, not a player.
		targets = []state.Target{{Player: c.Controller, IsPlayer: true}}
	}
	if digZone := strings.TrimSpace(sa.Params["DigZone"]); digZone != "" && !strings.EqualFold(digZone, "Library") {
		// The PlanarDeck carriers (4): planes are unimplemented engine-wide
		// and there is no planar-deck zone to scan. Loud, and nothing moves.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "DigUntil DigZone$ " + digZone + " is not implemented; no cards are revealed or moved"})
		return
	}
	// declineDest is where the found card goes when the optional move is
	// declined: OptionalNoDestination$ when the SA carries one, else the
	// revealed pile (the corpus oracles' "put all cards revealed this way
	// that weren't put onto the battlefield ...").
	declineDest := revDest
	if raw := strings.TrimSpace(sa.Params["OptionalNoDestination"]); raw != "" {
		declineDest = ParseZone(raw)
	}
	for _, t := range targets {
		p := PlayerOf(h, c, t)
		lib := zoneOf(g, state.ZLibrary, p)
		if len(lib) == 0 {
			continue
		}
		var revealed, found []state.ObjID
		for _, id := range lib {
			revealed = append(revealed, id)
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				found = append(found, id)
				if int32(len(found)) >= amount {
					break
				}
			}
		}
		// The reveal is PUBLIC (the same non-Secret ids-Note effDig's Reveal$
		// arm emits), recorded before the ask, once per resolution -- a
		// re-entry after the optional-move answer must not reveal again.
		if !moveDone {
			h.Emit(events.Event{Kind: events.Note, Player: p, IDs: revealed})
		}
		if optionalMove && !moveDone {
			verb := digDestPhrase(foundDest)
			d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
				Source:     c.Source,
				ResumeKind: "diguntil_move", ResumeSA: sa,
				Prompt: "Put the revealed matching card(s) onto " + verb + "?",
				Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: "Yes — put into " + verb, Player: p},
					{Index: 1, Kind: "no", Label: "No", Player: p},
				}}
			if Ask(h, d) == AskAsked {
				return // resolution suspended; the answer re-enters with Ctx.DigUntilMove set.
			}
			// Fuzz/no-engine host: the deterministic decline (R-9) — the found
			// card(s) join the decline destination.
			moveDone = true
			moveAns = "no"
		}
		switch {
		case rememberRevealed:
			// The revealed set already carries every found card (it is a
			// prefix scan), so RememberFound$ adds nothing new.
			for _, id := range revealed {
				c.Remembered = append(c.Remembered, state.Target{Obj: id})
			}
		case rememberFound:
			for _, id := range found {
				c.Remembered = append(c.Remembered, state.Target{Obj: id})
			}
		}
		foundJoinedRevealed := false
		if !noMoveFound && len(found) > 0 {
			dest := foundDest
			if optionalMove && moveAns != "yes" {
				dest = declineDest
				foundJoinedRevealed = dest == revDest
			}
			for _, id := range found {
				if dest == state.ZBattlefield {
					// An AURA face put onto the battlefield by a non-cast effect
					// never got a cast-time target. With NO eligible bearer the
					// card stays in the library (CR 303.4f's
					// remain-in-current-zone) instead of entering unattached and
					// dying to the CR 704.5m SBA.
					bearer, isAuraFace := auraEntryBearer(g, id, p)
					if isAuraFace && bearer == 0 {
						h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: p,
							Text: "no permanent the revealed Aura can enchant; it stays in the library (CR 303.4f)"})
						continue
					}
					ev := moveZoneEvent(c, id, state.ZLibrary, dest)
					ev.Player, ev.Secret = p, true
					h.Emit(ev)
					if tapped {
						h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: p, Text: "entered tapped"})
					}
					if gainControl {
						h.Emit(events.Event{Kind: events.ControlChange, Obj: id, Player: c.Controller})
					}
					if bearer != 0 {
						// The CR 303.4f attach, degraded to the deterministic
						// stand-in documented above (the bearer the scan picked).
						h.Emit(events.Event{Kind: events.Attach, Obj: id, IDs: []state.ObjID{bearer}})
					}
					// StaticEffect$ on a DigUntil battlefield take: the same
					// rider registration every ChangeZone mover applies (no
					// corpus carrier rides a DigUntil today; hooked so the
					// class cannot miss one).
					applyStaticEffect(h, c, sa, dest, []state.ObjID{id})
					continue
				}
				ev := moveZoneEvent(c, id, state.ZLibrary, dest)
				ev.Player, ev.Secret = p, true
				h.Emit(ev)
			}
		}
		// The revealed rest (plus a found card whose decline joined the pile)
		// move to RevealedDestination$; NoMoveRevealed$ True (8 corpus lines)
		// leaves them where they are instead.
		if noMoveRevealed {
			continue
		}
		for _, id := range revealed {
			isFound := false
			for _, fid := range found {
				if fid == id {
					isFound = true
					break
				}
			}
			if isFound && !foundJoinedRevealed {
				continue
			}
			if revDest == state.ZLibrary {
				// Library placement: "-1" (bottom) is a real library-to-library
				// move (Move's zone append lands it at the bottom — the exact
				// contract effDig's LibraryPosition2$ "-1" arm documents);
				// "0"/absent is the engine's stay-in-place default (the cards
				// already sit on top in their existing relative order) so no
				// event; anything else is named loudly and the card stays.
				if revPos == "-1" {
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZLibrary, To: state.ZLibrary, Player: p, Secret: true})
				} else if revPos != "" && revPos != "0" {
					h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: p,
						Text: "RevealedLibraryPosition$ " + revPos + " is not implemented; the card stays on top"})
				}
				continue
			}
			ev := moveZoneEvent(c, id, state.ZLibrary, revDest)
			ev.Player, ev.Secret = p, true
			h.Emit(ev)
		}
	}
}

// digUntilParamValue is the withhold-list keys' trimmed value read (the
// paramcensus's dynamic-key rule: the key is this helper's own parameter,
// and every call site passes a string literal). Empty means absent-or-False.
func digUntilParamValue(sa *cards.SA, key string) string {
	v := strings.TrimSpace(sa.Params[key])
	if strings.EqualFold(v, "False") {
		return ""
	}
	return v
}

// auraEntryBearer resolves a non-cast battlefield entry's Aura bearer: the
// first permanent in the entering controller's battlefield zone order that
// satisfies the face's Enchant keyword spec (or any permanent when the face
// carries no Enchant keyword — nothing in the corpus prints one, the same
// convention rules/attach.go's auraStillMatchesEnchant uses). aura is false
// when the face is not an Aura (no attach needed); aura && bearer == 0
// means the face IS an Aura but no eligible bearer exists. The filter read
// is the shared MatchesSpecFrom, so a compound spec (Enchant:
// Creature.YouCtrl) evaluates exactly like an attach-time legality check.
func auraEntryBearer(g *state.Game, id state.ObjID, p state.PlayerID) (state.ObjID, bool) {
	o := g.Obj(id)
	if o == nil || o.Face() == nil {
		return 0, false
	}
	isAura := false
	for _, t := range o.Face().Types {
		if strings.EqualFold(t, "Aura") {
			isAura = true
			break
		}
	}
	if !isAura {
		return 0, false
	}
	spec := "Permanent"
	if param, ok := o.Face().KeywordParam("Enchant"); ok && strings.TrimSpace(param) != "" {
		spec, _, _ = strings.Cut(param, ":")
	}
	spec = strings.TrimSpace(spec)
	for _, bid := range g.Zone(state.ZBattlefield, p) {
		if bid == id {
			continue
		}
		if MatchesSpecFrom(g, spec, bid, p, id) {
			return bid, true
		}
	}
	return 0, true
}

// effReveal backs Reveal, RevealHand and PeekAndReveal, which the brief
// specifies as one row sharing a single Amount param (NumCards, default 1)
// and one behaviour: reveal cards without disturbing them, recorded via a
// non-Secret Note carrying their identities so view projection stops
// redacting them. PeekAndReveal looks at the library; Reveal and RevealHand
// look at hand.
//
// Look$ True (28 corpus RevealHand lines — Gitaxian Probe, Glasses of
// Urza, Slayer's Bounty) turns the reveal into a PRIVATE look (CR 701.20e:
// a card looked at is shown only to the player the effect specifies — here
// the activator, not every seat): the note becomes a Secret Note scoped to
// the looker through emitLook, the one private-look channel every
// looker-scoped effect shares. The public reveal (no Look$) is unchanged.
//
// RevealHand's Forge semantics act on the WHOLE hand ("look at target
// player's hand", "target opponent reveals their hand" — both shapes are
// whole-hand), so when its SA carries NO NumCards$ parameter the amount is
// the pool's entire size, not the shared default 1 (task revealhand1:
// Gitaxian Probe revealed exactly one card of a seven-card hand). The key's
// PRESENCE, not its value, is the switch — a future script writing NumCards$
// wins — and the switch keys on API so Reveal (a card-selector family:
// RevealValid$/Defined$ picking specific cards; 6 of its 85 raw corpus
// lines carry NumCards$) and PeekAndReveal keep their count behaviour. The
// corpus carries ZERO NumCards$ on RevealHand (81 raw lines), so every
// compiled RevealHand SA today takes the whole hand. The pool is only known
// inside the walk, so the whole-hand amount is applied per target.
func effReveal(h Host, c *Ctx, sa *cards.SA) {
	_, hasNum := sa.Params["NumCards"]
	wholeHand := sa.API == "RevealHand" && !hasNum
	amt := int32(1)
	if hasNum {
		amt = Num(h, c, sa, "NumCards", 1)
		if amt < 0 {
			amt = 0
		}
	}
	zone := state.ZHand
	if sa.API == "PeekAndReveal" {
		zone = state.ZLibrary
		// PeekAmount$ (Herald's Horn, the Kinship family): how many library
		// cards the peek LOOKS at. The reveal below then covers the subset of
		// that window RevealValid$ admits, so the amount and the valid-filter
		// compose ("look at the top card; if it's a creature card of the
		// chosen type, you may reveal it"). An unresolvable value degrades to
		// zero through Num's present-but-unresolvable convention, which for
		// a peek means an empty window and no reveal ask -- the correct
		// reading of a count this build cannot compute.
		amt = Num(h, c, sa, "PeekAmount", 1)
		if amt < 0 {
			amt = 0
		}
	}
	// The may-reveal ask (task fb-3f1cc033, Delver of Secrets' peek; widened
	// to Optional$ by the round-2 review's Look$ task): the deciding player
	// is asked whether to reveal before the Note goes out. The ask is the
	// same mid-resolution vocabulary every other asking primitive uses —
	// KChoose yes/no with a ResumeKind, the answer re-entering effReveal
	// through rules' resumeResolution with Ctx.RevealOpt set. A host that
	// cannot ask (an effects-package double, fuzz) keeps the pre-ask
	// behaviour: the mandatory reveal, as the deterministic fallback (the
	// same R-9 degradation Scry/Surveil carry). Still unread here,
	// deliberately: NoReveal$/NoPeek$ and RememberRevealedPlayer$ — see the
	// report's Issues section. PeekAmount$ and RevealValid$ ARE read (the
	// PeekAndReveal arm above takes the peek window from PeekAmount$; the
	// RevealValid$ filter below narrows the may-reveal to the matching
	// subset for every API in this row).
	answer := c.RevealOpt
	c.RevealOpt = "" // fx42 scoping: consumed once; a nested peek poses its own ask
	// The bare-look ack (lookack): consumed once per WALK, together with its
	// per-target cursor — the answer attaches to the exact Defined$ target
	// that asked (the decision's ResumeTarget). Targets before the cursor
	// were fully processed on the pass that suspended and are skipped, the
	// cursor target emits without re-asking, and every LATER bare look in
	// the walk poses its own ack. Consuming at walk entry (fx42) also keeps
	// a nested bare look below this walk posing its own instead of
	// inheriting the answer.
	lookAck := c.LookAck
	lookAckTarget := c.LookAckTarget
	c.LookAck, c.LookAckTarget = false, 0
	look := strings.EqualFold(strings.TrimSpace(sa.Params["Look"]), "True")
	revealType := strings.TrimSpace(sa.Params["RevealType"])
	// The may-reveal ask: PeekAndReveal poses it through RevealOptional$
	// (Delver of Secrets); the Reveal/RevealHand shapes pose it through
	// Optional$ ("you may reveal" — Liar's Pendulum's two RevealHand lines
	// and the corpus's five Reveal lines), which used to be ignored and the
	// reveal forced. No corpus line combines Look$ with either flag, but the
	// ask is shaped to work for one anyway: a look is asked of the LOOKER
	// (the activator gains the information), a reveal of the player whose
	// cards would be shown.
	optional := strings.EqualFold(strings.TrimSpace(sa.Params["RevealOptional"]), "True") ||
		strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True")
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberRevealed"]), "True")
	random := strings.EqualFold(strings.TrimSpace(sa.Params["Random"]), "True")
	g := h.Game()
	// Forge's RevealDefined$ is the reveal family's equivalent of Defined$.
	// Copy the SA and translate only the target selector, so the common
	// resolver owns every Self/Targeted/Remembered spelling without mutating
	// the shared compiled corpus. This matters for opening-hand reveals:
	// Chancellor of the Tangle must reveal the chosen Chancellor, not an
	// unrelated first card in its controller's hand.
	revealSA := *sa
	if spec := sa.Params["RevealDefined"]; spec != "" {
		revealSA.Params = make(map[string]string, len(sa.Params)+1)
		for key, value := range sa.Params {
			revealSA.Params[key] = value
		}
		revealSA.Params["Defined"] = spec
	}
	for targetIndex, t := range Defined(h, c, &revealSA) {
		if lookAck && targetIndex < lookAckTarget {
			// The cursor skip: this target was fully processed (note emitted,
			// RememberRevealed$ captured) on an earlier pass of this same
			// resume chain, before the walk suspended on a later target's
			// ack — re-running it would duplicate its events.
			continue
		}
		p := PlayerOf(h, c, t)
		pool := zoneOf(g, zone, p)
		if sa.Params["RevealDefined"] != "" && !t.IsPlayer {
			// A RevealDefined object is itself the card to reveal, not a
			// selector for the first card in that player's zone.
			pool = []state.ObjID{t.Obj}
		}
		if revealType != "" {
			// RevealType$ (Slayer's Bounty: "look at the creature cards in
			// target opponent's hand") narrows the pool to the cards of that
			// type before any count is taken — Forge's RevealHandEffect
			// filters the hand by RevealType the same way. An unresolvable
			// spec matches nothing (the filter's fail-closed convention), so
			// a look/reveal over an unknown type shows nothing rather than
			// everything.
			filtered := make([]state.ObjID, 0, len(pool))
			for _, id := range pool {
				if MatchesSpecCtx(g, revealType, id, c.SpecContext(c.Controller)) {
					filtered = append(filtered, id)
				}
			}
			pool = filtered
		}
		if rv := strings.TrimSpace(sa.Params["RevealValid"]); rv != "" {
			// RevealValid$ (Herald's Horn's Creature.ChosenType, the Kinship
			// family's Card.sharesCreatureTypeWith): the may-reveal covers
			// only the pool's matching subset — "if it's a creature card of
			// the chosen type, you may reveal it". With nothing matching there
			// is no reveal and no RememberRevealed$ capture, so a chained
			// ConditionDefined$ Remembered gate correctly skips; the same
			// fail-closed convention RevealType$ applies above.
			filtered := make([]state.ObjID, 0, len(pool))
			for _, id := range pool {
				if MatchesSpecCtx(g, rv, id, c.SpecContext(c.Controller)) {
					filtered = append(filtered, id)
				}
			}
			pool = filtered
		}
		n := amt
		if wholeHand || int32(len(pool)) < n {
			n = int32(len(pool))
		}
		if random && len(pool) > 0 {
			// Random$ True (Urza's Bauble: "Look at a card at random in target
			// player's hand"): the pool narrows to n DISTINCT random cards,
			// drawn from the engine's seeded generator through Host.Rand — the
			// same deterministic source choose_control's AtRandom/Random
			// discards use — so the pick replays identically. The narrowing
			// runs AFTER the count is resolved (Rise // Fall's NumCards$ 2
			// reveals two, not one); a partial Fisher-Yates over a copy picks
			// the n cards without repeating one. For n==1 the shuffle's first
			// swap is exactly the old single h.Rand(len(pool)) pick, so every
			// seeded chain that ran the one-card shape replays byte-identically.
			// The pick is a LOOK, not a reveal: the NoReveal$ arm below is the
			// corpus's carrier (Urza's Bauble reveals nothing of what it saw —
			// the activator alone learns the card), and the public Note below
			// is skipped for it.
			rp := append([]state.ObjID(nil), pool...)
			for i := 0; i < int(n); i++ {
				j := i + h.Rand(len(rp)-i)
				rp[i], rp[j] = rp[j], rp[i]
			}
			pool = rp[:n]
		}
		if n == 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(sa.Params["NoReveal"]), "True") {
			// NoReveal$ True (Mishra's Bauble: "Look at the top card of target
			// player's library" — a look, never a reveal): the identity goes
			// to the ACTIVATOR alone through emitLook, the one private-look
			// channel (CR 701.20e's shown-only-to-the-looker rule), and the
			// public Note the reveal path emits does not happen. RememberRevealed$
			// finds nothing — a chained gate correctly does not fire; the
			// corpus's NoReveal$ carriers chain zone-less riders (Mishra's
			// slowtrip DelayedTrigger) that do not read the walked Remembered.
			//
			// The look is also the one information transfer with no decision
			// attached, which a client's auto-passing priority streams past
			// unread — the pacing defect the reporter hit. The bare look now
			// gates on the look_ack ack FIRST (ask-first: ask → suspend → the
			// resume arm sets Ctx.LookAck → the re-entered walk lands the note
			// below the modal). A Random$ narrowing re-derives from the seeded
			// generator on every pass, so a random bare look would show the
			// resume a DIFFERENT card than the prompt named — it keeps the
			// ungated shape (measured at the corpus pin: ZERO Reveal-family
			// lines combine Random$ with NoReveal$/Look$; the one Random$
			// carrier, Urza's Bauble, is the public-reveal path). The ack is
			// addressed by the per-target cursor (LookAckTarget, the decision's
			// ResumeTarget): the cursor target emits without re-asking, and a
			// later bare look in the same walk poses its own ack — consuming
			// the flag at the FIRST bare-look target instead would leave the
			// later target's ack unanswered and loop forever.
			if lookAck && targetIndex == lookAckTarget {
				// The answered target: its ack's resume pass re-entered here, so
				// fall through to the emit without re-asking.
			} else if !random {
				if poseLookAck(h, c, sa, c.Controller, p, zone, pool[:n], targetIndex) {
					return
				}
				// No host to ask (R-9), or the ask was skipped: fall through
				// and emit the look immediately — information is never lost to
				// a host that cannot ask, the same deterministic degradation
				// Scry/Surveil carry.
			}
			emitLook(h, []state.PlayerID{c.Controller}, zone, pool[:n], "")
			continue
		}
		asker := p
		if look {
			asker = c.Controller
		}
		if optional && answer == "" {
			// The peek ask's wording and payload are byte-stable: a golden
			// game (Delver of Secrets) poses exactly this ask.
			var prompt, yesLabel string
			options := []decision.Option{
				{Index: 0, Kind: "yes", Label: "", Player: asker},
				{Index: 1, Kind: "no", Label: "No", Player: asker},
			}
			if sa.API == "PeekAndReveal" && !look {
				// The ask must carry WHAT is being revealed: the peeking player is
				// deciding whether to reveal a card only they can see, and the
				// library is not projected to that seat (view exposes only
				// LibrarySize), so a count-only prompt asks a blind question.
				// The card names go into the prompt and the yes option's label,
				// and the top card rides the option's Obj — the same private
				// channel the hidden-library "search" options use (view.project
				// attaches a decision only to its own Decision.Player, so this
				// payload reaches the peeking seat alone; even an Omniscient
				// spectator gets no decision).
				names := make([]string, 0, n)
				for _, id := range pool[:n] {
					if o := g.Obj(id); o != nil && o.Face() != nil {
						names = append(names, o.Face().Name)
					}
				}
				prompt = "Reveal the top " + strconv.Itoa(int(n)) + " card(s) of your library?"
				if len(names) > 0 {
					prompt = "Reveal the top " + strconv.Itoa(int(n)) + " card(s) of your library — " + strings.Join(names, ", ") + "?"
				}
				yesLabel = "Yes — reveal"
				if len(names) == 1 {
					yesLabel = "Yes — reveal " + names[0]
				}
				options[0].Obj = pool[0]
			} else {
				// The decider already owns what is being decided over (a hand
				// reveal asks its owner; a look asks its looker), so the ask
				// carries no payload of its own — what "yes" later emits
				// reaches exactly the seats the shape allows.
				if look {
					prompt, yesLabel = "Look at the target player's hand?", "Yes — look"
				} else {
					prompt, yesLabel = "Reveal your hand?", "Yes — reveal"
				}
			}
			options[0].Label = yesLabel
			d := &decision.Decision{Player: asker, Kind: decision.KChoose, Min: 1, Max: 1,
				ResumeKind: "reveal_optional", ResumeSA: sa, Source: c.Source,
				Prompt:  prompt,
				Options: options}
			if Ask(h, d) == AskAsked {
				return
			}
			// No host to ask (R-9), or the ask was skipped: fall through to the
			// mandatory reveal below, deterministic run to run. (A
			// reveal_optional decision is Min == Max == 1 over two options, so
			// AskEmpty is unreachable by construction; the shared helper owns
			// the guard either way.)
		}
		if optional && answer == "no" {
			// Declined: no Note, and RememberRevealed$ finds nothing —
			// a chained gate (Delver's ConditionDefined$ Remembered)
			// correctly does not fire. The walk continues.
			continue
		}
		revealed := append([]state.ObjID(nil), pool[:n]...)
		if look {
			// CR 701.20e: a card looked at this way is shown only to the
			// player the effect specifies — the activator — so the record is
			// a Secret Note scoped to the looker (emitLook), NOT the public
			// Note the pre-fix build emitted here (the round-2 review's
			// Gitaxian Probe leak: every seat and spectator read the target's
			// whole hand off it). RememberRevealed$ below still sees the
			// looked-at cards: the chained subs that read Remembered are part
			// of the same walk the looker's own card drives.
			//
			// The mandatory look is gated on the look_ack ack (lookack) like
			// the NoReveal$ arm above — ask-first, the note lands on the
			// resume pass, addressed by the same per-target cursor. The
			// Look$+Optional$ combination keeps the reveal_optional ask ALONE:
			// the player who just answered "yes — look" has consented to the
			// follow-through, so no second gate (the brief's scope boundary;
			// measured at the corpus pin: zero corpus lines combine Look$ with
			// Optional$/RevealOptional$).
			if lookAck && targetIndex == lookAckTarget {
				// The answered target: its ack's resume pass re-entered here, so
				// fall through to the emit without re-asking.
			} else if !optional && !random {
				// The same Random$ re-derivation guard the NoReveal$ arm
				// carries (measured: zero corpus lines combine Random$ with
				// Look$, so the guard is dormant groundwork).
				if poseLookAck(h, c, sa, asker, p, zone, revealed, targetIndex) {
					return
				}
				// R-9: no host to ask — emit immediately, deterministically.
			}
			emitLook(h, []state.PlayerID{asker}, zone, revealed, "")
		} else {
			// No Text: the Note's payload is the ids, and view.Describe renders
			// them ("player 0 reveals Mountain #82") — defect 1's second half,
			// the client's only data path for hidden-zone ids in a reveal. A
			// Text-carrying Note would need the names baked in at emit time,
			// duplicating Describe's obj() naming; an empty Text with ids keeps
			// the naming in one place. Ruling T23-w still passes the Note
			// through RedactEvents unchanged (it is non-Secret).
			h.Emit(events.Event{Kind: events.Note, Player: p, IDs: revealed})
			// The opening-hand RevealCard ability of Impatient Iguana carries
			// this flag. The public reveal happened, so its "If you do" clause
			// takes effect as a replayed state transition; a declined optional
			// reveal reaches the continue above and cannot change the starter.
			if strings.EqualFold(strings.TrimSpace(sa.Params["BecomeStartingPlayer"]), "True") {
				h.Emit(events.Event{Kind: events.StartingPlayerChange, Player: c.Controller})
			}
		}
		if remember {
			// RememberRevealed$ (task fb-3f1cc033): the revealed cards join
			// the walk's Remembered set, where a chained ConditionDefined$
			// Remembered gate (Delver's transform) reads them. Fresh backing
			// array: on an ability resume Ctx.Remembered aliases the stack
			// object's own Remembered slice, and appending in place would
			// write shared state without an event. Measured at the corpus
			// pin: of the 67 PeekAndReveal+RememberRevealed SVar lines, 43
			// have downstream subs that read Remembered — all of them
			// condition gates or Defined$ Remembered bodies that Forge
			// itself intends to see the reveal (the Kinship family), so
			// inheriting the reveal here is the semantics, not a leak.
			next := make([]state.Target, 0, len(c.Remembered)+len(revealed))
			next = append(next, c.Remembered...)
			for _, id := range revealed {
				next = append(next, state.Target{Obj: id})
			}
			c.Remembered = next
		}
	}
}

// effRearrangeTopOfLibrary looks at the top NumCards of Defined$'s library
// and poses a KArrange decision over them: the player picks the order, and
// rules' handleArrange applies it as an events.LibraryOrder (Ruling J0/J1
// - the one general decision shape Scry, Surveil and Dig later share).
//
// Unlike effReveal's Note (a deliberate reveal, public to every seat), the
// private-look record is a Secret Note: only p, the library's own owner, may
// know what sat on top. Ruling T23-w makes a Note public by default
// (view.RedactEvents' rule 3 exempts Note entirely, on the theory that a
// Note IS the engine's "tell everyone" channel), so the one Note that must
// stay private has to opt OUT by being Secret -- the same shape
// rules/engine.go's Shuffle and this file's own effDraw already use for
// their own hidden-zone payloads. The Note records the look, not the order;
// the order that follows is the player's to choose.
//
// Min == Max == k (pile B is empty for a full reorder), one option per top
// card in top-down order, each Option.Kind "bottom" (nothing goes there for
// a reorder, but the vocabulary stays uniform so Scry/Surveil reuse it
// unchanged). Decision.Player is p, the library's owner — the player who is
// looking at and reordering their own top cards.
func effRearrangeTopOfLibrary(h Host, c *Ctx, sa *cards.SA) {
	// MayShuffle$ True (Ponder's "You may shuffle."): after the arrange is
	// applied, an optional shuffle. The ask is posed on the arrange RE-ENTRY
	// pass only -- the arrangement has been applied by then, so the shuffle
	// question asks about a settled library. The answer flows back through
	// the "arrange_mayshuffle" resume arm, which emits the Shuffle event
	// itself and re-enters this effect with both Arrange and MayShuffle set;
	// the MayShuffle done-marker (consumed and cleared here, fx42 scoping)
	// keeps that third pass from posing the ask again.
	mayShuffle := strings.EqualFold(strings.TrimSpace(sa.Params["MayShuffle"]), "True")
	// Re-entry after rules' handleArrange applied the answered KArrange and
	// emitted the LibraryOrder event: this pass must only let the resolution
	// continue (the chained SubAbility$ runs), not re-ask or re-emit. Clear
	// the marker so a nested arrange — the fx42 class of leak — cannot read
	// an outer arrange's "done".
	if c.Arrange {
		c.Arrange = false
		if mayShuffle && c.MayShuffle == "" {
			for _, t := range actingPlayers(h, c, sa) {
				p := PlayerOf(h, c, t)
				d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
					Source: c.Source, ResumeKind: "arrange_mayshuffle", ResumeSA: sa,
					Prompt: "Shuffle your library?",
					Options: []decision.Option{
						{Index: 0, Kind: "yes", Label: "Yes — shuffle", Player: p},
						{Index: 1, Kind: "no", Label: "No — keep the order", Player: p},
					}}
				if Ask(h, d) == AskAsked {
					return // resolution suspended; the answer re-enters with Ctx.MayShuffle set.
				}
				// Fuzz/no-engine host: the deterministic stand-in keeps the
				// order (declines the shuffle, R-9).
			}
		}
		if mayShuffle {
			c.MayShuffle = ""
		}
		return
	}
	n := Num(h, c, sa, "NumCards", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, t := range actingPlayers(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := zoneOf(g, state.ZLibrary, p)
		k := n
		if int32(len(lib)) < k {
			k = int32(len(lib))
		}
		emitLook(h, []state.PlayerID{p}, state.ZLibrary, nil, "looks at the top of the library")
		d := &decision.Decision{Player: p, Kind: decision.KArrange,
			Min:        int(k),
			Max:        int(k),
			Source:     c.Source,
			ResumeKind: "arrange",
			ResumeSA:   sa,
			Prompt:     "Rearrange the top " + strconv.Itoa(int(k)) + " card(s); the first card you pick goes on top"}
		for i := int32(0); i < k; i++ {
			name := "a card"
			if o := g.Obj(lib[i]); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: int(i),
				Kind: "bottom", Label: name, Obj: lib[i], Player: p})
		}
		// The shared ask boundary (effects.Ask) refuses to post a KArrange
		// whose only legal answer is the empty one: with an empty library (or
		// NumCards$ 0) k is 0, Min == Max == 0 and there are no options -- the
		// exact wedge shape. AskEmpty (and AskNoHost alike) resolves through
		// the stand-in below: the order is (re)set unchanged and the
		// resolution completes.
		if Ask(h, d) == AskAsked {
			return // resolution suspended; the answer re-enters with Ctx.Arrange set.
		}
		// Fuzz/no-engine host: the deterministic stand-in keeps the existing
		// order -- pile A = the offered options in offered order (J3) -- with
		// the LibraryOrder recording that the order was (re)set unchanged.
		h.Emit(events.Event{Kind: events.LibraryOrder, Player: p,
			IDs:    append([]state.ObjID(nil), lib...),
			Secret: true})
	}
}

// effScry implements the Scry prompt API (CR 701.18): look at the top
// ScryNum$ cards of Defined$'s library, put any number of them -- the ones
// the player does NOT pick -- on the BOTTOM of the library in the order they
// were offered, and the rest back on top in the order the player picks.
// It is the KArrange ask with Min 0, Max N and Option.Kind "bottom".
//
// The look is recorded as a Secret Note (the player alone may know what sat
// on top), then the answer is applied by rules' handleArrange, which routes
// the unchosen pile B to the destination named by the shared Kind
// ("bottom"). Re-entry after handleArrange set Ctx.Arrange must only let
// the chained SubAbility$ run, never re-ask -- the same done-marker
// discipline effRearrangeTopOfLibrary uses. The no-host stand-in (R-9)
// keeps every card on top in its existing order (pile B empty), which is
// narrower than the card text but deterministic.
func effScry(h Host, c *Ctx, sa *cards.SA) {
	// ScryNum$ is read HERE, at the api:Scry implementation, not inside the
	// shared KArrange body: the shared body takes the resolved count, so the
	// parameter read is a literal-key read on this API's own SA (Kozilek's
	// Command's `ScryNum$ X` Charm mode resolves the announced X through the
	// same Num grammar a literal would take).
	n := Num(h, c, sa, "ScryNum", 1)
	effLookAndArrange(h, c, sa, n, "bottom", "Scry", false)
}

// effSurveil implements the Surveil prompt API (CR 701.42): look at the top
// Amount$ cards of Defined$'s library, put any number of them -- the ones
// the player does NOT pick -- into the GRAVEYARD (surveil has no bottom
// pile; the fsv1 survey's bottom-pile reading is wrong and this code follows
// CR 701.42), and the rest back on top in the order the player picks. It is
// the KArrange ask with Min 0, Max N and Option.Kind "graveyard".
//
// Re-entry and the no-host stand-in are exactly effScry's (the same shared
// helper): the stand-in puts nothing in the graveyard, which is narrower
// than the card text but deterministic.
func effSurveil(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "Amount", 1)
	effLookAndArrange(h, c, sa, n, "graveyard", "Surveil", true)
}

// effLookAndArrange is the shared KArrange body behind effScry and
// effSurveil: the count (ScryNum$ / Amount$, default 1) is resolved by the
// calling api implementation through Num; this body resolves Defined$ (default = the ability's source, hence its controller),
// and pose one KArrange decision per target library over the top min(N,
// len(lib)) cards. The unchosen pile B's destination is the shared Option.Kind
// passed in; only that differs between the two primitives.
//
// markSurveil selects the one verb-specific record: Surveil emits ONE
// events.Surveil marker per acting player -- the canonical record
// trig:Surveil matches ("whenever you surveil" -- Mirko, Obsessive Theorist;
// Dimir Spybug; Thoughtbound Phantasm; Whispering Snitch) -- while Scry
// emits none. The marker is emitted INSIDE the per-player loop, at the
// point that player's arrangement is actually performed, NOT for every
// defined target up front: a multi-player `Defined$` Surveil poses only the
// FIRST library's KArrange (the documented multi-library Scry/Surveil
// limitation), so emitting for every target before the loop queued surveil
// triggers for players who never surveilled (fb: an opponent's Whispering
// Snitch fired for a player whose library was untouched). A suspended first
// player's re-entry (Ctx.Arrange set) returns before the loop, so its marker
// is not re-emitted; the no-host stand-in and the continuation passes both
// keep the marker already emitted for the player the loop reached.
func effLookAndArrange(h Host, c *Ctx, sa *cards.SA, n int32, kind, verb string, markSurveil bool) {
	// Re-entry after rules' handleArrange applied the answered KArrange and
	// emitted the LibraryOrder event: this pass must only let the resolution
	// continue (the chained SubAbility$ runs), not re-ask or re-emit.
	if c.Arrange {
		c.Arrange = false
		return
	}
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, t := range actingPlayers(h, c, sa) {
		p := PlayerOf(h, c, t)
		if markSurveil {
			h.Emit(events.Event{Kind: events.Surveil, Player: p, Obj: c.Source})
		}
		lib := zoneOf(g, state.ZLibrary, p)
		k := n
		if int32(len(lib)) < k {
			k = int32(len(lib))
		}
		emitLook(h, []state.PlayerID{p}, state.ZLibrary, nil, "looks at the top of the library")
		d := &decision.Decision{Player: p, Kind: decision.KArrange,
			Min:        0,
			Max:        int(k),
			Source:     c.Source,
			ResumeKind: "arrange",
			ResumeSA:   sa,
			Prompt:     verb + " " + strconv.Itoa(int(k)) + ": pick the cards to keep on top, in order; the rest go to " + destinationPhrase(kind)}
		for i := int32(0); i < k; i++ {
			name := "a card"
			if o := g.Obj(lib[i]); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: int(i),
				Kind: kind, Label: name, Obj: lib[i], Player: p})
		}
		// The shared ask boundary (effects.Ask) refuses to post a KArrange
		// whose only legal answer is the empty one: with an empty library (or
		// ScryNum$/SurveilNum$ 0) k is 0, Min 0 / Max 0 and there are no
		// options -- the exact wedge shape. AskEmpty (and AskNoHost alike)
		// resolves through the stand-in below: every zero cards keep their
		// place and the resolution completes.
		if Ask(h, d) == AskAsked {
			return // resolution suspended; the answer re-enters with Ctx.Arrange set.
		}
		// Fuzz/no-engine host: the deterministic stand-in keeps every card
		// on top in its existing order (pile B empty for a Scry, nothing to
		// the graveyard for a Surveil), with the LibraryOrder recording that
		// the order was (re)set unchanged.
		h.Emit(events.Event{Kind: events.LibraryOrder, Player: p,
			IDs:    append([]state.ObjID(nil), lib...),
			Secret: true})
	}
}

// destinationPhrase names pile B's destination in the human-readable prompt;
// it mirrors the Option.Kind vocabulary so the prompt and the wire never
// disagree.
func destinationPhrase(kind string) string {
	switch kind {
	case "bottom":
		return "the bottom of your library"
	case "graveyard":
		return "your graveyard"
	case "exile":
		return "exile"
	case "hand":
		return "your hand"
	default:
		return "their destination"
	}
}

// effNameCard records a card-name choice. The real name is asked at cast
// time and recorded with a Choose event before this ever resolves (plan
// ruling R-6), so a source already carrying ChosenName is a no-op. Without
// one -- a script that uses NameCard outside an ETB replacement -- it names
// the first card in the controller's library, which is at least a
// deterministic, legal name for whatever downstream sub-ability expects
// one, recorded now as the Choose event so the choice survives replay.
// effHideaway implements CR 702.75: when a permanent with Hideaway enters,
// its controller looks at the top N cards, exiles one face down, then puts
// the rest on the bottom in the order they chose. Exile provenance is carried
// by MoveZone, so a later Play resolves Defined$ ExiledWith by identity.
func effHideaway(h Host, c *Ctx, sa *cards.SA) {
	if c.HideawayArranged {
		c.HideawayArranged = false
		return
	}
	g := h.Game()
	if c.HideawayPicked {
		// The choice was recorded by rules' hideaway_pick resume arm. Move the
		// selected card before arranging: that leaves precisely the remaining
		// cards at the top of the library for the KArrange handler.
		id := c.Hideaway
		c.Hideaway = 0
		c.HideawayPicked = false
		lib := zoneOf(g, state.ZLibrary, c.Controller)
		if id == 0 || !containsObj(lib, id) {
			return
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile,
			Counter: "exiled_with_face_down", Amount: int32(c.Source), Secret: true})
		hideawayBottom(h, c, sa)
		return
	}
	n := int(Num(h, c, sa, "Amount", 4))
	if n < 0 {
		n = 0
	}
	lib := zoneOf(g, state.ZLibrary, c.Controller)
	if n > len(lib) {
		n = len(lib)
	}
	if n == 0 {
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
		Source: c.Source, ResumeKind: "hideaway_pick", ResumeSA: sa,
		Prompt: "Choose a card to exile with Hideaway"}
	for i, id := range lib[:n] {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "hideaway", Label: objName(g, id), Obj: id, Player: c.Controller})
	}
	if h.Ask(d) {
		return
	}
	// The no-host degradation chooses the first card, then retains the offered
	// order for the rest on the bottom.
	h.Emit(events.Event{Kind: events.MoveZone, Obj: lib[0], From: state.ZLibrary, To: state.ZExile,
		Counter: "exiled_with_face_down", Amount: int32(c.Source), Secret: true})
	hideawayBottom(h, c, sa)
}

func hideawayBottom(h Host, c *Ctx, sa *cards.SA) {
	lib := zoneOf(h.Game(), state.ZLibrary, c.Controller)
	n := int(Num(h, c, sa, "Amount", 4)) - 1
	if n < 0 {
		n = 0
	}
	if n > len(lib) {
		n = len(lib)
	}
	if n == 0 {
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KArrange, Min: n, Max: n,
		Source: c.Source, ResumeKind: "hideaway_arrange", ResumeSA: sa,
		Prompt: "Put the remaining Hideaway cards on the bottom in any order"}
	for i, id := range lib[:n] {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "hideaway_bottom", Label: objName(h.Game(), id), Obj: id, Player: c.Controller})
	}
	if h.Ask(d) {
		return
	}
	newLib := append(append([]state.ObjID(nil), lib[n:]...), lib[:n]...)
	h.Emit(events.Event{Kind: events.LibraryOrder, Player: c.Controller, IDs: newLib, Secret: true})
}

func containsObj(ids []state.ObjID, want state.ObjID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func effNameCard(h Host, c *Ctx, sa *cards.SA) {
	if o := h.Game().Obj(c.Source); o != nil && o.ChosenName != "" {
		return
	}
	g := h.Game()
	name := "a card"
	if lib := zoneOf(g, state.ZLibrary, c.Controller); len(lib) > 0 {
		if o := g.Obj(lib[0]); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "name", Text: name})
}

// discardDefinedCards resolves a Discard SA's DefinedCards$ parameter to the
// concrete objects it names. Only the group spellings the corpus's Mode$
// Defined discards actually use are wired (Remembered and its aliases, the
// targets); any other spelling falls through the generic Defined resolver.
func discardDefinedCards(h Host, c *Ctx, spec string) []state.Target {
	switch strings.Split(spec, ".")[0] {
	case "Remembered", "RememberedLKI", "RememberedCard", "DirectRemembered":
		return objectsOf(c.Remembered)
	case "Targeted":
		return objectsOf(c.Targets)
	}
	return Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})
}
