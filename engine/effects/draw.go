package effects

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// effDraw is api:Draw's resolution. Every parameter of its own it reads comes
// from the compiled DrawParams (draw_params.go), the one reader of a Draw
// ability's parameters; internal/codeshape's drawParamLeaks ratchet holds
// this file to no parameter read at all. Whose draws (actingPlayers) is the
// shared Defined$/ValidTgts$ selector tier.
func effDraw(h Host, c *Ctx, sa *cards.SA) {
	dp := DrawOf(sa)
	noteUnreadParams(h, c, "Draw", dp.Unread)
	n := numText(h, c, dp.NumCards, 1)
	if n <= 0 {
		return
	}
	// RememberDrawn$ records every card actually drawn into the resolution's
	// Remembered (Breathstealer's Crypt's reveal-and-maybe-discard chain acts
	// on exactly the drawn card; a library that ran out mid-draw records only
	// what moved). Unread before this — the whole sub-chain saw nothing. A
	// suspended draw has not happened yet, so the record waits until the
	// draw is real (the suspend check below).
	// The corpus uses both True and AllReplaced. True records only the card
	// this ability's own draw moved (the pre-draw top card). AllReplaced also
	// records the cards a replacement body drew in its place: CR 121.6c/614.1
	// says a replaced draw is not drawn but the replacement instruction's
	// cards ARE, and Forge's AllReplaced marker adds every card the draw
	// machinery moves to hand under that cause (see recordDrawn below).
	remember := dp.RememberDrawn
	// tapeReentry repeats the unread-parameter Note and the capture
	// exclusion after each answer served in place (the event stream the
	// answered path has always emitted).
	tapeReentry := func() {
		noteUnreadParams(h, c, "Draw", dp.Unread)
		if remember {
			c.Remembered = rememberedExcludingCapture(h, c)
		}
	}
	if remember {
		// A triggered ability's resolution starts with its fire-time event
		// capture already in Ctx.Remembered (engine/rules/resolution.go seeds both
		// Remembered and Captured from the ability object's own Remembered).
		// That object -- for Communal Brewing's self-ETB trigger, the
		// entering Brewing itself -- is not a card drawn this way, so it must
		// not inflate Remembered$Amount ("one ingredient counter ... for each
		// card drawn this way") nor defeat the did-I-draw-anything gate
		// (Mr. Foxglove's `ConditionDefined$ Remembered | ConditionCompare$
		// EQ0`). Drop it before recording what the draws actually moved; the
		// helper is the same capture-exclusion every TriggerRemembered$Amount
		// read uses, and it is a no-op for an activated ability (no capture).
		c.Remembered = rememberedExcludingCapture(h, c)
	}
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
	// in place (ResumeKind "draw_optional"); no answer keeps the pre-ask
	// mandatory draw (the R-9 degradation). A
	// spec the grammar cannot resolve fails closed below this read's own
	// convention: the pre-ask mandatory draw stays and one loud Note names
	// the unmodelled value. A target with an empty library makes the draw a
	// non-choice — Forge's canDrawAmount guard skips those silently, so the
	// ask only fires when SOME target could actually draw; with none, no
	// question is posed and nothing is drawn (an empty-library draw event is
	// a no-op either way).
	if decider := dp.OptionalDecider; decider != "" && total > 0 {
		answered := ""
		if c.Draw.Done > 0 {
			// Draws already made: draws happen only after the decider said
			// yes, so the election is already made -- re-posing it would ask
			// again and, on a second yes, restart the draws from zero.
			answered = "yes"
		}
		if answered == "" {
			canDraw := false
			for _, t := range targets {
				if len(zoneOf(h.Game(), state.ZLibrary, t)) > 0 {
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
					resolved := DefinedSpec(h, c, spec)
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
					if ans, ok := AskTape(h, d); ok {
						answered = "no"
						if answerYes(ans) {
							answered = "yes"
						}
						tapeReentry()
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
	// Decision.Player, so no leak). Answered in place (ResumeKind
	// "draw_upto"), the count tracked per target in Ctx.Draw. No answer
	// keeps the pre-ask mandatory draw (the R-9 degradation every
	// effDraw arm takes). A library with fewer than n cards caps the ask at
	// what is there; an empty library is a no-op (never a decision whose
	// only answer is empty — OnlyEmptyAnswer refuses it — and never a
	// mandatory draw event that would mill a player the card only offered
	// to draw).
	if dp.Upto {
		for idx := int(c.Draw.UptoIdx); idx < len(targets); idx++ {
			p := targets[idx]
			if !c.Draw.UptoAnswered {
				lib := zoneOf(h.Game(), state.ZLibrary, p)
				m := n
				if int32(len(lib)) < m {
					m = int32(len(lib))
				}
				if m <= 0 {
					c.Draw.UptoIdx = int32(idx + 1)
					continue
				}
				d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 0, Max: int(m),
					ResumeKind: "draw_upto", ResumeSA: sa, ResumeTarget: idx, Source: c.Source,
					Prompt: "Draw up to " + strconv.Itoa(int(n)) + " card(s)?"}
				for i := int32(0); i < m; i++ {
					id := lib[i]
					label := "a card"
					if o := h.Game().Obj(id); o != nil && o.Face() != nil {
						label = o.Face().Name
					}
					d.Options = append(d.Options, decision.Option{Index: len(d.Options),
						Kind: "card", Label: label, Obj: id, Player: p})
				}
				if ans, ok := AskTape(h, d); ok {
					// The answered count, drawn for this target now.
					c.Draw.UptoCount, c.Draw.UptoAnswered = int32(len(ans)), true
					tapeReentry()
				} else {
					// No answer (R-9): the pre-ask mandatory draw of what was offered.
					c.Draw.UptoCount, c.Draw.UptoAnswered = m, true
				}
			}
			for c.Draw.Done < c.Draw.UptoCount {
				// The chosen count is this target's batch: offer it to the
				// DrawCards replacement class before the first card moves
				// (the same boundary the mandatory loop takes).
				if c.Draw.Done == 0 && drawBatchConsumed(h, c.Source, p, c.Draw.UptoCount, c, dp.RememberDrawnAll) {
					c.Draw.Done = c.Draw.UptoCount
					if h.Suspended() {
						return
					}
					break
				}
				var lib []state.ObjID
				if remember {
					lib = zoneOf(h.Game(), state.ZLibrary, p)
				}
				before := drawnSnapshot(h, p, dp.RememberDrawnAll)
				if drawFor(h, p, int(c.Draw.Done), sa, drawUptoRider{idx: idx, count: c.Draw.UptoCount}) {
					// A Dredge answer, already applied: on past this draw.
					tapeReentry()
					c.Draw.Done++
					continue
				}
				if h.Suspended() {
					// The resolution suspended between individual draws; do
					// not run later targets yet.
					return
				}
				if remember {
					recordDrawn(h, c, p, before, lib, dp.RememberDrawnAll)
				}
				c.Draw.Done++
			}
			c.Draw.Done = 0
			c.Draw.UptoIdx = int32(idx + 1)
			c.Draw.UptoCount, c.Draw.UptoAnswered = 0, false
		}
		return
	}
	for c.Draw.Done < total {
		p := targets[c.Draw.Done/n]
		// The batch boundary (Forge's drawCards(n) DrawCards pass): the whole
		// multi-card batch is offered to the DrawCards replacement class
		// before any card moves, so Alms Collector's "would draw two or more
		// cards" replaces a Thought Reflection draw-two as ONE event. A
		// consumed batch drew through its replacement's body already; the
		// cursor moves past it and nothing is drawn here.
		if c.Draw.Done%n == 0 && drawBatchConsumed(h, c.Source, p, n, c, dp.RememberDrawnAll) {
			c.Draw.Done += n
			if h.Suspended() {
				return
			}
			continue
		}
		var lib []state.ObjID
		if remember {
			lib = zoneOf(h.Game(), state.ZLibrary, p)
		}
		before := drawnSnapshot(h, p, dp.RememberDrawnAll)
		if drawFor(h, p, int(c.Draw.Done), sa, drawUptoRider{}) {
			// A Dredge answer, already applied: on past this draw.
			tapeReentry()
			c.Draw.Done++
			continue
		}
		if h.Suspended() {
			// The resolution suspended between individual draws; do not run
			// later draws, Remembered or SubAbility$ yet.
			return
		}
		if remember {
			recordDrawn(h, c, p, before, lib, dp.RememberDrawnAll)
		}
		c.Draw.Done++
	}
	// The draw cursor is scoped to this primitive.
	c.Draw.Done = 0
}

// drawnSnapshot copies the drawing player's library before a draw when the
// cause is RememberDrawn$ AllReplaced: the cards the draw and its replacement
// body move into that player's hand are then read back from it. nil for the
// True spelling, whose single pre-draw top card is read from the live zone
// slice the caller already holds (the fold replaces the slice, so it keeps
// the pre-draw contents).
func drawnSnapshot(h Host, p state.PlayerID, all bool) []state.ObjID {
	if !all {
		return nil
	}
	return append([]state.ObjID(nil), zoneOf(h.Game(), state.ZLibrary, p)...)
}

// recordDrawn appends one draw instruction's remember records. AllReplaced
// records every card of the pre-draw snapshot now in the DRAWING player's own
// hand, in library order -- the draw itself and whatever its replacement body
// drew (CR 121.6c/614.1; Forge's AllReplaced marker). A card an opponent's
// replacement drew (Notion Thief) is in the opponent's hand, not p's, and is
// not recorded. True keeps recording only the pre-draw top card.
func recordDrawn(h Host, c *Ctx, p state.PlayerID, before, top []state.ObjID, all bool) {
	if !all {
		if len(top) > 0 {
			c.Remembered = append(c.Remembered, state.Target{Obj: top[0]})
		}
		return
	}
	hand := zoneOf(h.Game(), state.ZHand, p)
	for _, id := range before {
		if slices.Contains(hand, id) {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
	}
}

// drawBatchConsumed offers one draw instruction's batch to the DrawCards
// replacement class and reports whether a replacement consumed it. When one
// did, its body drew in the batch's place, so a RememberDrawn$ AllReplaced
// cause records those draws exactly as the per-card loop records its own
// (the snapshot is taken before the body runs).
func drawBatchConsumed(h Host, source state.ObjID, p state.PlayerID, n int32, c *Ctx, rememberAll bool) bool {
	before := drawnSnapshot(h, p, rememberAll)
	if !proposeDrawBatch(h, source, p, n) {
		return false
	}
	if !h.Suspended() {
		recordDrawn(h, c, p, before, nil, rememberAll)
	}
	return true
}

// proposeDrawBatch offers one draw instruction's whole batch to the
// DrawCards replacement class before any card moves (Forge's Player.
// drawCards(n) DrawCards pass; XMage's DRAW_TWO_OR_MORE_CARDS). handled
// reports the batch was consumed -- a replacement's body already drew in
// this instruction's place, which the host's held-proposal seam reports as
// Amount 0 -- so the caller draws nothing for the batch. The proposal is
// held, never logged, and the host's own replacement registry decides it.
func proposeDrawBatch(h Host, source state.ObjID, p state.PlayerID, n int32) bool {
	if n <= 0 {
		return false
	}
	got := h.CountReplacementProposed(events.Event{Kind: events.DrawBatchProposal,
		Player: p, Obj: source, Amount: n})
	// A surviving batch keeps its positive count (or a body's rewrite of it);
	// a consumed one reports 0, whatever event shape the consumption took.
	return got.Amount == 0
}
