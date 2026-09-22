package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func (e *Engine) beginTurn(active state.PlayerID, skipUntap ...bool) {
	e.emit(events.Event{Kind: events.TurnChange, Player: active, Amount: e.G.Turn + 1})
	// CR 500.7 riders apply to the particular queued extra turn, not every
	// later turn of its controller. The variadic form keeps ordinary callers
	// explicit-free while advanceStep supplies the pending grant's SkipUntap.
	// A skipped untap step never emits an Untap event for anything, and the
	// turn structure jumps straight to upkeep, so its turn-based actions
	// (Suspend's TIME decrement, ...) still run on schedule.
	step := state.StepUntap
	if len(skipUntap) > 0 && skipUntap[0] {
		step = state.StepUpkeep
	}
	prior := e.pending
	e.setStep(step)
	if e.pending != nil && e.pending != prior {
		// An Optional$ BeginPhase replacement parked the entry. Its answer
		// calls finishEnteredStep after entering or skipping the step.
		return
	}
	e.finishEnteredStep()
}

// finishEnteredStep performs the turn-based action owed by the step that a
// StepChange just entered (or by the landing step of one or more skipped
// steps), then grants priority. Both ordinary transitions and an answered
// Optional$ BeginPhase replacement use this one continuation, so declining
// Fasting enters and draws exactly once while accepting it lands in main1
// without drawing. Untap is special: after its action the turn enters upkeep
// before priority; a replacement may park either entry, in which case the
// eventual answer resumes this helper again.
// chooseSuspendCast is the chooseFor for CR 702.62a's may-cast ask, posed by
// startSuspendedCast when a suspended card's last TIME counter is removed.
// iota+12 is pairwise distinct from the shared package set (cast=1/etb=2/
// miracle=3, cleanup=4, division=5, mana=6..9, opening=10); the exact numbers
// only need to differ.
const chooseSuspendCast chooseFor = iota + 12

// chooseUntap is the per-permanent untap-step election. It is deliberately
// a KChoose (rather than a priority action): the controller answers before
// the turn-based Untap event is emitted.
const chooseUntap chooseFor = 40

func (e *Engine) finishEnteredStep() {
	if e.G.Step == state.StepUntap && !e.finishUntapStep(0) {
		return
	}
	if e.G.Step == state.StepUpkeep {
		// CR 702.62: only a card that entered exile through the Suspend action
		// loses TIME counters. The final-counter trigger then casts it if able;
		// it is not an optional priority action and arbitrary exiled Suspend
		// cards never acquire that permission. Gated on the step actually
		// entered, so a BeginPhase replacement that skipped the upkeep step
		// (landing directly on the draw) does not decrement.
		for _, id := range e.G.Zone(state.ZExile, e.G.Active) {
			o := e.G.Obj(id)
			if o == nil || o.Counter("TIME") <= 0 {
				continue
			}
			if o.CastFlags&state.FlagSuspend == 0 && !o.SuspendGranted {
				// Only a card that entered exile through the Suspend action,
				// or received a real Suspend grant while in exile, loses TIME
				// counters. A plotted card carries none -- CR 701.34's timing
				// is "on a later turn", not an upkeep count (rules/legal.go's
				// exile walk reads Object.PlottedTurn).
				continue
			}
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -1})
			if o.Counter("TIME") == 0 {
				e.suspendedCasts = append(e.suspendedCasts, id)
			}
		}
		if e.startSuspendedCast() {
			return
		}
	}
	// An upkeep skip can land the turn directly on the draw step, whose
	// turn-based action must still run (CR 504.1 -- the skip took the upkeep
	// step, never the draw's draw). drawStepTurnAction is the one entry to
	// that action, shared with advanceStep's ordinary path.
	if e.G.Step == state.StepDraw && e.drawStepTurnAction() {
		return
	}
	// Entry resets the pass count along with the active holder. Cumulative
	// upkeep is a real Phase trigger expanded from its keyword, so the upkeep
	// StepChange queued it alongside every other upkeep trigger; the ordinary
	// priority round orders and places them before anyone may act.
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

// untapStep is the remaining deterministic battlefield scan after a
// replacement-order decision parks one Untap event. It belongs to the queued
// replacement choice, not Game state: the final selected replacement is what
// the log records, and replay reaches the same turn-based scan naturally.
type untapStep struct {
	next int
}

// finishUntapStep performs the untap turn-based action from next onward. An
// Untap replacement competition can suspend on one permanent; its answered
// choice resumes at the following permanent, rather than advancing to upkeep
// while the choice is pending or re-processing the already replaced event.
// stat:UntapOtherPlayer: after the active player's own battlefield, EVERY
// other living player's battlefield permanent a matching static admits untaps
// too (CR's "untap during each other player's untap step", and the
// command-zone plane shape "all permanents untap during each player's untap
// step"); AliveFrom(0) order keeps the event stream deterministic. No
// repl:Untap replacement can park on those foreign untaps: every corpus line
// scopes itself with ValidStepTurnToController$ You, which the matcher reads
// against the untapped card's own controller, and a foreign card's controller
// is not the step's active player.
func (e *Engine) finishUntapStep(next int) bool {
	ids := e.G.Zone(state.ZBattlefield, e.G.Active)
	for i := next; i < len(ids); i++ {
		o := e.G.Obj(ids[i])
		if o == nil {
			continue
		}
		if o.ExertSkipUntap {
			// CR 702.100b (task exert1): an exerted creature won't untap
			// during its controller's next untap step. The window closes
			// here -- consumed at use, the regeneration-shield precedent: the
			// Amount -1 Exert event's fold clears the flag, and replay
			// re-derives both the skip and the consume from the same scan.
			// Untap EFFECTS are deliberately untouched: CR 702.100b names
			// only the untap step, so this gate lives in the turn scan and
			// never in effects.TryUntap (a Combat-Celebrant untap-all still
			// untaps an exerted creature). An already-untapped permanent's
			// window is consumed just the same: the next untap step has
			// passed either way. (The stat:UntapOtherPlayer foreign scan
			// below neither skips nor consumes: the flag's owner is the
			// permanent's controller, whose own untap step is the active
			// scan this loop walks.)
			e.emit(events.Event{Kind: events.Exert, Obj: ids[i], Amount: -1})
			continue
		}
		if !o.Tapped {
			continue
		}
		if hasUntapStepChoice(o) && o.UntapChoice == "" {
			// CR 502.2: the controller may elect not to untap this
			// permanent. Option 0 is the deterministic untap/default path;
			// option 1 keeps it tapped. The answer is logged through Choose
			// before the scan continues, so replay and clones agree.
			e.untapResume = &untapStep{next: i + 1}
			e.untapChoiceObj = o.ID
			e.choosing = chooseUntap
			e.ask(&decision.Decision{Player: o.Controller, Kind: decision.KChoose,
				Min: 1, Max: 1, Source: o.ID,
				Prompt: "Untap this permanent?",
				Options: []decision.Option{
					{Index: 0, Kind: "untap", Obj: o.ID, Label: "Untap"},
					{Index: 1, Kind: "keep_tapped", Obj: o.ID, Label: "Keep tapped"},
				}})
			return false
		}
		e.untapResume = &untapStep{next: i + 1}
		prior := e.pending
		// effects.TryUntap, reached through untapTurnPermanent, applies the
		// shared stun-counter replacement (CR 122.1d) ahead of the raw Untap
		// event the repl:Untap replacement competition may park on.
		e.untapTurnPermanent(ids[i])
		if e.pending != nil && e.pending != prior {
			// poseUntapReplacementChoice transferred this continuation to its
			// queue entry. Do not enter upkeep until its answer finishes this
			// scan, and do not retain transient state across the pending intent.
			e.untapResume = nil
			return false
		}
		e.untapResume = nil
	}
	for _, p := range e.G.AliveFrom(0) {
		if p == e.G.Active {
			continue
		}
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o != nil && o.Tapped && e.untapOtherStaticsMatch(id) {
				e.untapTurnPermanent(id)
			}
		}
	}
	e.setStep(state.StepUpkeep)
	return e.pending == nil
}

// drawStepTurnAction runs the draw step's turn-based action (CR 504.1) when
// the engine has just entered StepDraw -- from advanceStep's ordinary step
// advance, or from beginTurn after a BeginPhase replacement skipped the
// upkeep step -- and reports whether the game ended with it (an
// empty-library draw is a loss), telling the caller not to emit a further
// Priority event. Exactly the pre-refactor advanceStep gate, stated once so
// both entries cannot drift:
//
//	CR 504.1: the draw step's draw is a TURN-BASED ACTION -- it happens
//	once, automatically, at the beginning of the step, before any player
//	receives priority, full stop. It is not conditioned on priority state
//	in any way.
//
//	Ruling T23-x: before this, the draw lived in priorityRound, gated on
//	`Passes == 0 && Priority == Active` -- a PROXY for "the step just
//	began" that Task 23's own test author measured is also exactly the
//	state resolveTop's callers restore after every resolution (CR 117.3b --
//	Priority{Player: e.G.Active, Amount: 0}; see the T14-e comments in
//	stack.go / legal.go). So a mandatory "whenever you draw a card" trigger
//	that resolved during the draw step made the proxy true again, drew a
//	SECOND card, queued a second trigger, and so on until the library ran
//	out: one seat drawing 20 cards inside what the log still called one
//	step, the other seat never getting a turn. Keying the draw on the step
//	being ENTERED, instead of on ambient Passes/Priority state that anything
//	resolving later in the step can also produce, makes it run exactly once
//	no matter what resolves afterward.
//
//	Ruling F45: CR 103.8a skips the starting player's first draw only in a
//	two-player game. Multiplayer free-for-all games take that draw normally
//	(CR 800.7). len(e.G.Players) is the constructed seat count; eliminated
//	players remain in the slice, so the rule cannot change as players lose.
//	e.G.Turn > 1 preserves the draw on every later turn. !Lost keeps an
//	eliminated active player from drawing.
//
//	Ruling T28-b (fix round 1): the Lost guard is REACHABLE in ordinary
//	play, not a defensive leftover -- an earlier draft of this comment
//	called it "unreachable today" on the theory that an empty-library draw
//	was the only way to become Lost before this point, and Task 22 already
//	falsified that: any state-based action can eliminate the active player
//	during their OWN turn, before their OWN draw step, for a reason that
//	has nothing to do with drawing at all (CR 704.5a life loss is the
//	common case). Measured: an upkeep self-drain (`Mode$ Phase | Phase$
//	Upkeep`) trigger (this repo's own drainerSrc fuzz fixture) eliminates
//	its controller during that seat's turn-2 upkeep with their library
//	still full, and turn 2's draw step is then entered with the eliminated
//	seat still Active. The turn structure does not skip steps for an
//	eliminated active player, only priority -- so their draw step is still
//	entered, and this is what stops it from drawing on their behalf. A
//	reader who trusts "unreachable" here is invited to delete this guard,
//	and deleting it is exactly the mutant that draws for an eliminated
//	player.
//
//	The draw can also SUSPEND on a mid-draw ask: a Dredge replacement
//	(CR 702.55) poses its KModes choice through the same DrawFor this
//	turn-based action shares with the Draw primitive, and the ask leaves
//	e.pending set (exactly the condition the Advance loop pauses on).
//	CR 405.1: the step's priority comes only AFTER the turn-based action
//	completes -- and the dredge answer's resume path (resolution.go's
//	dredge arm -> Advance -> priorityRound) grants that one priority
//	itself. Reporting the suspension here (e.pending != nil alongside
//	e.G.Over) is what keeps the emit below from granting priority twice
//	and from logging a Priority event before the player had even answered
//	whether to replace the draw (findings-sol4 MAJOR;
//	dredge_turn_draw_test.go is the committed probe).
//
// CR 702.151a (Sagas, kw:Chapter): "As this Saga enters and after your draw
// step, add a lore counter." The ETB half is granted in events.Move (the
// same every-entry-site convention the planeswalker starting loyalty uses);
// advanceSagas here is the after-your-draw-step half -- one lore counter per
// Saga the ACTIVE player controls, once per turn, after the draw. The
// chapter triggers queue off the CounterChange events it emits (rules' chapter
// check). Skipped when the draw itself ended the game (e.G.Over), mirroring
// every other post-state-change guard in this file.
func (e *Engine) drawStepTurnAction() bool {
	if e.G.Step != state.StepDraw || (len(e.G.Players) == 2 && e.G.Turn <= 1) ||
		e.G.Players[e.G.Active].Lost {
		return false
	}
	e.drawCard(e.G.Active)
	if e.G.Over {
		return true
	}
	if e.pending != nil {
		// The draw suspended on a mid-draw ask (a Dredge replacement's
		// KModes choice): the step's priority comes from the answer's
		// resume path (resolution.go's dredge arm -> Advance ->
		// priorityRound), so the emit below must not grant it twice. The
		// after-draw Saga grant is likewise deferred: it belongs after the
		// step's draw actually lands, and the dredge answer re-drives the
		// turn structure before that priority.
		return true
	}
	e.advanceSagas(e.G.Active)
	return false
}

// startSuspendedCast consumes the next final-counter trigger before anyone
// gets priority. A targetless/un-castable card is simply left in exile, the
// "if able" part of CR 702.62; a legal one is offered to its controller: CR
// 702.62a's cast is OPTIONAL ("you may cast it without paying its mana cost
// if able"), so the controller answers a real yes/no decision and a decline
// leaves the card in exile. A yes enters the ordinary no-cost cast flow and
// can still ask for targets.
func (e *Engine) startSuspendedCast() bool {
	for len(e.suspendedCasts) > 0 {
		id := e.suspendedCasts[0]
		e.suspendedCasts = e.suspendedCasts[1:]
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZExile ||
			(o.CastFlags&state.FlagSuspend == 0 && !o.SuspendGranted) || o.Face() == nil {
			continue
		}
		// "If able" includes every restriction that makes casting illegal,
		// not merely whether the spell can find a target. In particular a
		// CantBeCast static remains effective when Suspend supplies the mana
		// cost; beginning the cast and discovering the restriction afterwards
		// would incorrectly put the spell on the stack. An uncastable card is
		// never offered: there is nothing to choose (CR 702.62a casts "if
		// able"), so no decision is posed for it.
		if e.castRestricted(o.Owner, id) || !e.castTargetsAvailable(o.Owner, id, o.Face().SpellAbility()) {
			continue
		}
		name := "it"
		if f := o.Face(); f != nil && f.Name != "" {
			name = f.Name
		}
		e.choosing = chooseSuspendCast
		e.ask(decision.New(o.Owner, decision.KChoose, "Cast "+name+" without paying its mana cost?", 1, 1,
			[]decision.Option{{Index: 0, Kind: "suspend_cast_yes", Obj: id, Label: "Cast it"},
				{Index: 1, Kind: "suspend_cast_no", Obj: id, Label: "Leave it in exile"}}))
		return true
	}
	return false
}

// suspendCastAnswer applies CR 702.62a's may-cast answer. The offered card
// was popped from suspendedCasts when its ask was posed, so the answer's
// object is the only provenance this needs. A yes enters the ordinary cast
// flow (suspend_cast mode, no mana cost); a decline — or a card that left
// exile, changed hands or lost its Suspend provenance while the ask was
// outstanding — leaves the card in exile, which is exactly what CR 702.62a
// says a card whose cast was not made does. Remaining suspended casts, if
// any, are offered next; when none are, the caller's Advance loop resumes
// the step it was in (the same route the forced cast used after resolution).
func (e *Engine) suspendCastAnswer(chosen []decision.Option) {
	if len(chosen) == 0 {
		return
	}
	id := chosen[0].Obj
	o := e.G.Obj(id)
	if chosen[0].Kind == "suspend_cast_yes" && o != nil && o.Zone == state.ZExile &&
		(o.CastFlags&state.FlagSuspend != 0 || o.SuspendGranted) && o.Face() != nil {
		e.beginCast(o.Owner, decision.Option{Kind: "cast", Obj: id, Mode: "suspend_cast"})
		return
	}
	if e.pending == nil {
		// Declined (or the card is no longer a castable suspended card): the
		// offer is over, so the chooseFor it installed must not leak into the
		// next KChoose a different flow asks.
		e.choosing = chooseNone
		e.startSuspendedCast()
	}
}

func (e *Engine) setStep(s state.Step) {
	leaving := e.G.Step
	previous := e.stepLeaving
	e.stepLeaving = &leaving
	e.emit(events.Event{Kind: events.StepChange, Step: s})
	e.stepLeaving = previous
	if e.pending != nil {
		// Optional BeginPhase parked the transition. Boundary cleanup belongs
		// after that choice and is resumed by handleReplacement; emitting it
		// after DecisionAsk would mutate the game while a decision is pending.
		return
	}
	e.finishStepBoundary(leaving, s)
}

func (e *Engine) finishStepBoundary(leaving, entering state.Step) {
	// Mana pools empty as each step ends (CR 500.4). A live stat:UnspentMana
	// static protects a seat's unspent mana of the named colour: its keep
	// letters ride the event Text ("" = nothing protected, the historical
	// shape every game without a carrier emits) and the ManaClear fold honours
	// them, so the replay derives the same keep set from the same deterministic
	// static walk.
	for i := range e.G.Players {
		if e.G.Players[i].Pool.Total() > 0 {
			ev := events.Event{Kind: events.ManaClear, Player: state.PlayerID(i)}
			ev.Text = e.unspentManaKeep(state.PlayerID(i))
			e.emit(ev)
		}
	}
	if leaving == state.StepEndCombat && entering != leaving {
		// CR 702.109a exiles every Myriad token at end of combat before the
		// combat-state reset. Emit only when one exists, so unrelated combats
		// retain their established event stream while replay still folds the
		// same deterministic cleanup whenever it matters.
		for i := range e.G.Objs {
			if o := &e.G.Objs[i]; o.IsMyriad && o.Zone == state.ZBattlefield {
				e.emit(events.Event{Kind: events.MyriadCleanup})
				break
			}
		}
		// CR 511.3 removes creatures and planeswalkers from combat as the end
		// of combat step ends, not when it begins. Keeping the leaving-step
		// boundary here covers every transition made through setStep exactly
		// once; the former cleanup safety net would emit a duplicate reset.
		// Ruling T21-e keeps the reset event-sourced so a log-only replay also
		// learns that IsAttacking and BlockedBy were cleared.
		e.emit(events.Event{Kind: events.EndCombatReset})
		// CR 511.3: "until end of combat" control effects end with the step.
		e.expireControl(controlAtEndOfCombat)
		e.reconcileControlStatics()
	}
}

// step performs the smallest unit of automatic engine work.
func (e *Engine) step() {
	e.checkStateBased()
	if e.startSuspendedCast() {
		return
	}
	if e.G.Over {
		return
	}
	// The CR 903.9 commander replacement (Task m32) can leave a decision
	// pending from inside checkStateBased: a state-based action that moves a
	// commander parks the move and asks its owner, and checkStateBased
	// returns with that decision outstanding. The step switch below must not
	// then run -- askAttackers/priorityRound would hand out a SECOND,
	// unrelated decision and silently overwrite the parked commander's
	// (Advance pauses on the first e.pending regardless, so this guard is
	// what keeps the switch from clobbering it). Nothing else in the engine
	// leaves a pending decision after checkStateBased, so the guard is inert
	// for every pre-existing path.
	if e.pending != nil {
		return
	}
	// The CR 903.4b commander colour-choice round runs first: its answer must
	// exist before the mulligan/opening rounds and turn 1. Like the mulligan
	// round below, it must NOT leaf into the ordinary step switch.
	if e.coloring {
		e.stepColorRound()
		return
	}
	// The London mulligan round (Config.Mulligans > 0) runs between the
	// opening deal and turn 1. While e.pregame, stepPregame issues the single
	// next round decision; it must NOT leaf into the ordinary step switch,
	// whose cases assume a live turn with an active player. Once the round
	// hand to beginTurn it clears pregame itself.
	if e.pregame {
		e.stepPregame()
		return
	}
	switch e.G.Step {
	case state.StepDeclareAttackers:
		if e.declarationMadeThisStep(events.DeclareAttackers) {
			e.priorityRound()
		} else {
			e.askAttackers()
		}
	case state.StepDeclareBlockers:
		// A split attack is declared against one defender at a time. An
		// earlier DeclareBlockers event therefore does not finish the step
		// until the blocker-round cursor has exhausted every defender.
		if e.declarationMadeThisStep(events.DeclareBlockers) &&
			(e.blockerRound.order == nil || e.blockerRound.cursor >= len(e.blockerRound.order)) {
			e.blockerRound = blockerRound{}
			// The declare-blockers round is complete: every defender has
			// answered (or been skipped). This is the one instant
			// Mode$ AttackerUnblockedOnce's condition is evaluated -- see
			// checkAttackerUnblockedOnceTriggers. Queued here, the trigger
			// drains onto a stack at the priorityRound below (CR 509.2),
			// before combat damage.
			e.checkAttackerUnblockedOnceTriggers()
			e.priorityRound()
		} else {
			e.askBlockers()
		}
	case state.StepCombatDamage:
		e.combatStep()
	default:
		e.priorityRound()
	}
}

// declarationMadeThisStep derives a declaration step's completed substate
// from the replayable event log. A StepChange is the boundary: declarations
// from an earlier combat cannot satisfy the current step.
func (e *Engine) declarationMadeThisStep(kind events.Kind) bool {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.StepChange {
			return false
		}
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func (e *Engine) priorityRound() {
	// Nobody receives priority during untap or cleanup -- except on the
	// CR 514.3 grounds cleanupStep's caller below spells out.
	if e.G.Step == state.StepUntap || e.G.Step == state.StepCleanup {
		if e.G.Step == state.StepCleanup {
			// CR 514.1 (Task D1): the discard-down-to-maximum-hand-size
			// turn-based action runs FIRST, then CR 514.2. cleanupStep asks a
			// KChoose "discard" decision when the active player's hand is
			// over seven and returns with e.pending set; when it does, the
			// step SUSPENDS -- advanceStep must not hand to the next turn
			// while a player's discard choice is outstanding. The Advance
			// loop would pause on e.pending regardless, but returning here
			// keeps this round from advancing anyway; the answer resumes via
			// Submit -> handleChoose -> e.discardCleanup (combat.go), which
			// emits the discard moves and then finishes the step through the
			// same CR 514.3 tail as the no-discard path (finishCleanupStep).
			//
			// CR 514.2: cleanup removes damage and "until end of turn"
			// effects. Wired in here by Task 21 -- Engine.EndOfTurnCleanup
			// (layers.go) has existed since Task 19c, but nothing ever called
			// it, so a resolved pump effect used to survive forever instead
			// of expiring at the end of the turn it was cast in. This must
			// run before advanceStep, not after: cleanup logically belongs to
			// the step being left, not the one being entered, regardless of
			// what advanceStep itself does on the way into the next one.
			// (Ruling T23-x: this comment used to justify that by naming the
			// draw-step's own Passes-gated draw, which advanceStep's Passes
			// reset was load-bearing for; the draw is now a turn-based action
			// run unconditionally on entry to the step, so there is no such
			// gate left to protect -- but the ordering requirement here was
			// never actually ABOUT that gate, so it stands unchanged.)
			//
			// The 514.2 body runs either from cleanupStep directly (no
			// discard owed, the ordinary path with a hand of seven or fewer)
			// or from discardCleanup after the discard answer is recorded --
			// never from both, so no 514.2 action is ever done twice.
			e.repeatCleanup()
			return
		}
		e.advanceStep()
		return
	}
	// CR 117.5: state-based actions and triggered abilities are handled
	// before any player receives priority.
	//
	// Task 27: putTriggersOnStack can now ASK -- a controller with two or
	// more simultaneous triggers orders them, and an optional trigger's
	// decider says yes or no -- so it reports whether it finished. Returning
	// here on a true is load-bearing: it stops askPriority from immediately
	// overwriting e.pending with a second, unrelated decision. The drain
	// resumes through resumeTriggerDrain below, not by re-entering this
	// function -- see resumeTriggerDrain's own comment for why a fresh
	// checkStateBased has to run ahead of the resumed drain.
	//
	// (Ruling T23-x: this function used to also perform the draw step's own
	// draw here, gated on a Passes/Priority proxy for "the step just began"
	// -- see advanceStep -- and re-entering this function mid-round used to
	// draw a second card for exactly that reason: nothing between that draw
	// and the priority emit that followed it changed Passes or Priority. The
	// draw has moved to advanceStep, run once on entry to the step and never
	// again, so that specific risk is gone; resuming into grantPriority
	// rather than back through this function is still correct, for the
	// reason above, but is no longer load-bearing against a double draw.)
	if e.putTriggersOnStack() {
		return
	}
	e.grantPriority()
}

// grantPriority is the tail of a priority round: hand priority to whoever is
// due it and ask them what they want to do. Split out of priorityRound by Task
// 27 so that a trigger decision asked partway through the round has an exact
// continuation to resume into -- the part of the round that had not run yet,
// and nothing that had already run.
func (e *Engine) grantPriority() {
	if e.G.Over {
		// A state-based action during the drain can end the game. A finished
		// game must not hand out a fresh priority decision.
		return
	}
	holder := e.G.Priority
	if e.G.Players[holder].Lost {
		holder = e.G.NextAlive(holder)
	}
	e.emit(events.Event{Kind: events.Priority, Player: holder, Amount: e.G.Passes})
	e.askPriority(holder)
}

// repeatCleanup runs the cleanup procedure from its top: the CR 514.1
// discard and the CR 514.2 "until end of turn" body (cleanupStep), then the
// CR 514.3 tail (finishCleanupStep), which places any trigger waiting and
// hands out priority while it resolves or advances the turn.
//
// It is the ONE home for "do the cleanup step" and every entry into a
// cleanup procedure goes through it -- the ordinary priority round above,
// and (CR 514.3b) a cleanup-step priority round whose stack has just emptied.
// That second caller is the repeat the rules require: when a cleanup trigger
// resolves or a player casts an instant in the cleanup-step priority window,
// the 514.1/514.2 actions must run AGAIN before the turn can end, so an
// until-end-of-turn effect created by that instant expires and a hand pushed
// over the limit by that trigger is discarded. Routing the pass-case through
// here rather than straight to advanceStep is what makes that happen;
// without it the next turn simply begins.
func (e *Engine) repeatCleanup() {
	e.cleanupStep()
	if e.pending != nil {
		return
	}
	e.finishCleanupStep()
}

// finishCleanupStep is the CR 514.3 tail of the cleanup step, the one
// continuation shared by repeatCleanup (which also runs the 514.1/514.2
// actions ahead of it) and the answered-discard path (discardCleanup,
// combat.go). After the 514.1/514.2 turn-based actions have run, CR 514.3a
// places every trigger waiting (the state-based-action pass already happened
// at step()'s head) and gives the players priority while the stack is
// non-empty; when the stack empties, this function advances the turn.
//
// The CR 514.3b repeat that must run BEFORE that advance lives in the two
// callers, not here. When the step is entered normally, Advance -> step ->
// priorityRound reaches repeatCleanup again; when priority was granted from
// inside a cleanup priority round (finishCleanupStep's own grantPriority
// below, or resumeTriggerDrain's tail), the round is left through
// handlePriority's empty-stack pass, which routes back into repeatCleanup
// (legal.go). Both routes run cleanupStep once more and, finding no trigger
// and no stack, end the step. The loop terminates on the same property every
// priority round here does: triggers are finite and one-shot registrations
// are consumed at their DelayedPush.
//
// putTriggersOnStack's true return means it asked a decision (an ordering or
// an optional-trigger ask): e.pending is set and the drain resumes through
// resumeTriggerDrain, whose tail -- grantPriority, never a priorityRound
// re-entry -- finishes this interrupted round exactly as it finishes every
// other one. grantPriority's direct call below is the no-decision case: the
// drain placed triggers on the stack and it is simply the players' turn to
// respond (CR 117.1) while the step is still cleanup.
func (e *Engine) finishCleanupStep() {
	if e.putTriggersOnStack() {
		return
	}
	if len(e.G.Stack) > 0 {
		e.grantPriority()
		return
	}
	e.advanceStep()
}

// resumeTriggerDrain continues a half-drained trigger queue after one of its
// decisions has been answered, and finishes the interrupted priority round
// once the queue runs dry. It is the continuation for every decision that is
// asked from inside handle.
//
// The leading checkStateBased is fix round 1, review finding F1, and it is
// what makes this a faithful continuation rather than a shortcut. Before Task
// 27 EVERY decision was created from inside Advance -> step(), whose first
// statement is checkStateBased() -- so a decision was never handed out with
// state-based actions outstanding. This function runs from inside handle,
// which Submit calls BEFORE its own tail checkStateBased, and Advance is a
// no-op once handle has set e.pending. Without this line two things follow,
// both reproduced:
//
//   - CR 117.5 is violated: a creature a state-based action is about to sweep
//     is still alive when the drain finishes, so its death trigger reaches
//     the stack only after the priority holder has already acted, landing
//     above anything they cast (measured: stack = 2, queued = 1).
//   - The match hangs: priority is granted to a player the tail
//     checkStateBased then eliminates, and with three or more seats the game
//     does not end, so nothing can ever answer that decision.
//
// Running it here rather than inside grantPriority keeps it off
// priorityRound's own path, where step() has already reached the same fixed
// point -- a second call there would give a replacement-blocked state-based
// action a second attempt per step and change sba.go's measured firing counts
// (Ruling T22-p).
//
// This cannot recurse. checkStateBased's own tail calls
// releasePendingDecisionOfDepartedPlayer, which does nothing unless
// e.pending != nil, and every caller of this function has just cleared
// e.pending (Submit before handle, or that same release hook before calling
// here). The ask that sets e.pending again happens strictly afterwards.
//
// It also cannot re-offer a settled order. The queue re-entered below is the
// same one the answer just settled, and e.orderedTriggers suppresses the
// APNAP re-sort over that prefix; anything checkStateBased queues is appended
// behind it, exactly as for any other mid-drain arrival. Measured, not
// argued: TestTriggerDrainInvariantsUnderRandomizedPlay asserts across 120
// games that no ordering decision is ever offered twice with nothing placed
// in between.
func (e *Engine) resumeTriggerDrain() {
	// Fix round 2: the guard that makes termination a property of the
	// control flow rather than of an argument about callers.
	//
	// checkStateBased -> releasePendingDecisionOfDepartedPlayer ->
	// resumeTriggerDrain -> checkStateBased is a real cycle in the call
	// graph. Round 1 argued it cannot run away because all three callers
	// clear e.pending first, and the re-review confirmed that empirically
	// (sbaCalls=331491, sbaMaxDepth=2, resumeCalls=9961,
	// resumeWithPending=0) -- but also showed the argument is one
	// statement-reorder away from failing: swapping the release hook's
	// `e.pending = nil` and its call to this function makes recursion run
	// away immediately, and the failure mode is a stack overflow, i.e. a
	// totality violation. Task 22 is this repo's standing evidence for what
	// such an argument is worth (wrong four times out of four). One line
	// converts it into something the compiler's own control flow enforces
	// and no future reader has to re-derive.
	//
	// It is also the correct behaviour on its own terms: re-entering a drain
	// while a decision is outstanding would place triggers behind the
	// answering player's back and overwrite the very question they were
	// asked. TestResumeTriggerDrainIsInertWhileADecisionIsPending pins that.
	//
	// Task fx39 settled a competing suspicion: that the guard should be
	// e.Suspended() (e.resume != nil) instead of e.pending != nil. It is
	// NOT. The two are not the same state in principle -- a resolution can
	// be suspended with no decision pending for a moment, and a decision
	// can be pending with nothing suspended -- but on this engine they are
	// tied wherever a drain is resumed: e.resume is set only inside effects'
	// Ask (rules/resolution.go), which ALSO sets e.pending, and the drain's
	// resumed calls (handleTarget, handleTriggerOrder/Optional, handleChoose,
	// and releasePendingDecisionOfDepartedPlayer after the resumeResolution
	// it runs) all carry e.pending == nil with e.resume == nil, or both set.
	// So e.resume != nil implies e.pending != nil, and checking the latter
	// is strictly STRONGER: it also stops the drain from re-entering over a
	// pending NON-suspended decision (a trigger_order or target ask, a
	// priority round) -- exactly the outstanding-decision scenario above. A
	// guard on e.Suspended() alone would let that drain run and overwrite
	// the question, which the measured mutant confirms: swapping this line
	// to `if e.Suspended() { return }` fails
	// TestResumeTriggerDrainIsInertWhileADecisionIsPending by replacing the
	// pending trigger_order ask. Keep the guard on e.pending.
	if e.pending != nil {
		return
	}
	e.checkStateBased()
	if e.G.Over {
		return
	}
	if e.putTriggersOnStack() {
		return
	}
	e.grantPriority()
}

func (e *Engine) askPriority(p state.PlayerID) {
	d := &decision.Decision{
		Player: p, Kind: decision.KPriority, Min: 1, Max: 1,
		Prompt: fmt.Sprintf("turn %d, %s — %s has priority",
			e.G.Turn, e.G.Step, seatFacingName(e.G, p)),
		Options: e.legalActions(p),
	}
	e.ask(d)
}

func (e *Engine) advanceStep() {
	if e.G.Step == state.StepDeclareAttackers && e.declarationMadeThisStep(events.DeclareAttackers) {
		attacked := false
		for i := len(e.L.Events) - 1; i >= 0; i-- {
			ev := e.L.Events[i]
			if ev.Kind == events.StepChange {
				break
			}
			if ev.Kind == events.DeclareAttackers && len(ev.IDs) > 0 {
				attacked = true
				break
			}
		}
		if !attacked {
			// CR 508.8 skips blockers and combat damage, but only after the
			// CR 508.2 priority round in the declare-attackers step finishes.
			e.setStep(state.StepEndCombat)
			return
		}
	}
	if e.G.Step == state.StepCleanup {
		// CR 500.7: extra turns are taken in REVERSE order of creation (the
		// most recently created extra turn is taken first), inserted
		// immediately after the turn that created them, and the ordinary turn
		// order resumes only once every pending extra turn is spent. The
		// pending queue is the folded ExtraTurnQueue (events/apply.go); the
		// consumption is the -1 ExtraTurn event, which also carries the
		// grant's Final-Fortune-style rider (events.Apply registers the
		// delayed end-step trigger on it -- the granted turn is exactly the
		// turn about to begin). A grant to a seat that has since LOST takes
		// no turn: it is consumed (so the fold agrees) and skipped, and the
		// next pending grant, if any, is taken in the same cleanup.
		for len(e.G.ExtraTurnQueue) > 0 {
			grant := e.G.ExtraTurnQueue[len(e.G.ExtraTurnQueue)-1]
			seat := grant.Player
			if !e.G.Players[seat].Lost {
				// R:Event$ BeginTurn | ExtraTurn$ True | Skip$ True (Trouble in
				// Pairs, Stranglehold, Ugin's Nexus, Gerrard's Hourglass
				// Pendant): the granted seat skips the turn instead. The
				// consumption is still emitted so the ExtraTurnQueue/
				// ExtraTurns fold agrees, but WITHOUT the grant's
				// Final-Fortune rider -- the granted turn never begins, so its
				// delayed end-step trigger must not register -- and WITHOUT
				// the beginTurn: the loop moves on to the next pending grant
				// and, once the queue drains, the ordinary rotation below
				// resumes the turn order normally.
				skip, unsupported := e.extraTurnSkipped(seat)
				if unsupported {
					e.emit(events.Event{Kind: events.Note, Player: seat,
						Text: "R:Event$ BeginTurn ExtraTurn$ replacement matched with an unimplemented action (Skip$ absent or ReplaceWith$ present); the extra turn proceeds"})
				} else if skip {
					e.emit(events.Event{Kind: events.ExtraTurn, Player: seat, Amount: -1})
					continue
				}
			}
			obj, counter, phase := e.latestUnconsumedGrant(seat)
			e.emit(events.Event{Kind: events.ExtraTurn, Player: seat, Amount: -1,
				Obj: obj, Counter: counter, IDs: []state.ObjID{phase}})
			if !e.G.Players[seat].Lost {
				// beginTurn resets the pass count along with the repeated
				// holder; the ordinary rotation pointer does not advance.
				e.beginTurn(seat, grant.SkipUntap)
				return
			}
		}
		// No extra turn is pending: the next seat in the ordinary rotation is
		// the seat after the most recent NORMAL turn's holder (rotationBase) --
		// never after the seat that just finished an extra turn, whose turn
		// was inserted into the rotation, not a part of it.
		// beginTurn resets the pass count along with the new holder.
		e.beginTurn(e.G.NextAlive(e.rotationBase()))
		return
	}
	if e.G.Step == state.StepCombatDamage && e.combatRound.hasFirst && e.combatRound.firstDone && !e.combatRound.regularDone {
		// CR 510.3/4 (Task jj-cmb F37): the between-passes priority round just
		// completed, so the regular damage pass still owes before the step can
		// advance. Running it here may suspend again on a controller
		// damage-division decision (CR 510.1c); if it does, a decision is
		// pending and this function returns, and the answer resumes through
		// handleDamageDivision -> finishCombatPass, which moves the step to
		// end combat. If the pass completes here, finishCombatPass has
		// already set StepEndCombat, so return without double-advancing.
		e.beginCombatPass(false)
		return
	}
	next := e.G.Step + 1
	if s, ok := e.extraPhaseBoundary(); ok {
		next = s
	}
	e.setStep(next)
	if e.pending != nil {
		return
	}
	e.finishEnteredStep()
}

// extraPhaseBoundary is the extra-phase consumer (Forge AddPhaseEffect, DB$
// AddPhase; the state.Game.ExtraPhases fold is the queue, the ExtraTurnQueue
// precedent one level up). It is called at the ONLY site that advances the
// ordinary turn walk -- the tail of advanceStep -- and answers what step the
// walk enters next. The turn is treated as the phase LIST the grant spliced
// (state.ExtraPhase's type comment): when the walk leaves step L,
//
//  1. every CONSUMED grant whose extra phase ends at L (RangeEnd) completes
//     here -- one -2 ExtraPhase event per entry, the entry removed from the
//     fold;
//  2. then the earliest-created PENDING grant spliced at L -- or, when step
//     1 completed grants, at the same insertion point they spliced at
//     (Obeka's N upkeeps all splice after the same end-of-combat step, and
//     each next upkeep is spliced there, not after the last one) -- is
//     consumed (-1) and its extra phase entered;
//  3. when step 1 completed grants but step 2 found nothing to splice, the
//     walk jumps to the LAST completed grant's resume point (FollowedBy$, or
//     AfterStep+1 when absent);
//  4. otherwise the natural advance (Step+1) -- ok=false.
//
// The two list-splice jumps the walk makes OUTSIDE this site (CR 508.8's
// declare-attackers-with-no-attack jump to the end-of-combat step, and the
// combat-damage pass's jump there) skip the consumer: no corpus carrier
// splices at or completes across declare-attackers or combat-damage, and an
// extra combat reached through them still completes at its own RangeEnd
// (leaving end-of-combat goes through this site). A resume point outside the
// step range (or invalid) degrades to the natural advance, never a panic.
func (e *Engine) extraPhaseBoundary() (state.Step, bool) {
	leaving := e.G.Step
	// 1. Completing grants: one -2 event per consumed entry whose extra
	// phase ends here. emit folds synchronously, so the queue shrinks under
	// the loop -- rescan from the top after every completion. The LAST
	// completed grant's resume point is the walk's continuation (the
	// innermost completed extra phase is where the ordinary walk stands).
	completedAfter := state.Step(0)
	completed := false
	resume := state.Step(0)
	for {
		idx := -1
		for i := range e.G.ExtraPhases {
			if ep := e.G.ExtraPhases[i]; ep.Consumed && ep.RangeEnd == leaving {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		ep := e.G.ExtraPhases[idx]
		e.emit(consumedExtraPhaseEvent(ep, -2))
		completed, completedAfter = true, ep.AfterStep
		// The walk resumes at the explicit FollowedBy$ when the grant named
		// one, else at the phase that would naturally have followed the
		// splice point (AfterStep+1 -- Forge AddPhaseEffect's default).
		if ep.HasFollowedBy {
			resume = ep.FollowedBy
		} else {
			resume = ep.AfterStep + 1
		}
	}
	// 2. Splicing the next pending grant: earliest created first, spliced at
	// the leaving step -- or, when this boundary completed grants, at their
	// insertion point too.
	for i := range e.G.ExtraPhases {
		ep := e.G.ExtraPhases[i]
		if ep.Consumed {
			continue
		}
		if ep.AfterStep != leaving && !(completed && ep.AfterStep == completedAfter) {
			continue
		}
		e.emit(consumedExtraPhaseEvent(ep, -1))
		return ep.Entry, true
	}
	if completed && resume.Valid() && resume <= state.StepCleanup {
		return resume, true
	}
	return 0, false
}

// consumedExtraPhaseEvent builds the -1 (consume) / -2 (complete) message
// for one queue entry: the full identity the fold matches on (IDs[0] the
// entry step), with the delayed-trigger rider echoed in Text (the consume's
// registration reads it) and the follow/resume point deliberately absent --
// the fold's own entry already holds it, and a fixed slot beside the entry
// step could not be told apart from an absent rider.
func consumedExtraPhaseEvent(ep state.ExtraPhase, amount int32) events.Event {
	ev := events.Event{Kind: events.ExtraPhase, Player: ep.Player, Obj: ep.Source,
		Amount: amount, Step: ep.AfterStep, Counter: ep.Execute,
		IDs: []state.ObjID{state.ObjID(ep.Entry)}}
	if ep.HasDelayedPhase || ep.ValidPlayer != "" {
		ev.Text = events.EncodeExtraPhaseRiders(events.ExtraPhaseRiders{
			HasDelayedPhase: ep.HasDelayedPhase, DelayedPhase: ep.DelayedPhase,
			ValidPlayer: ep.ValidPlayer})
	}
	return ev
}

// handle dispatches a validated intent to the code that owns that decision
// kind. Later tasks add the combat and target cases.
func (e *Engine) handle(d *decision.Decision, in decision.Intent) {
	switch d.Kind {
	case decision.KPriority:
		e.handlePriority(d, in)
	case decision.KTarget:
		e.handleTarget(d, in)
	case decision.KAttackers:
		e.handleAttackers(d, in)
	case decision.KBlockers:
		e.handleBlockers(d, in)
	case decision.KTriggerOrder:
		e.handleTriggerOrder(d, in)
	case decision.KTriggerOptional:
		e.handleTriggerOptional(d, in)
	case decision.KCommanderZone:
		// The CR 903.9 "put it into the command zone instead" choice (Task
		// m32): the answered decision emits the parked MoveZone for real --
		// to the command zone on an accept, verbatim on a decline -- and
		// hands the queue to the next parked commander if any. See
		// handleCmdZone (rules/replacement.go).
		e.handleCmdZone(d, in)
	case decision.KReplacement:
		// The CR 616.1 order choice among competing replacement effects
		// (rules/replacement.go): the answered decision applies the chosen
		// replacement for real and hands the queue to the next parked
		// competition, if any.
		e.handleReplacement(d, in)
	case decision.KChoose:
		e.handleChoose(d, in)
	case decision.KMulligan:
		// The London mulligan round's keep/mulligan and bottoming asks
		// (rules/mulligan.go). pregame is set exactly while the round runs, so
		// handleMulligan is only ever reached with the round live.
		e.handleMulligan(d, in)
	case decision.KModes:
		// One handler serves cast-time mode announcements, triggered-ability
		// placement modes and mid-resolution asks; ResumeKind plus the trigger
		// drain flag distinguishes their continuations (rules/resolution.go).
		e.handleModes(d, in)
	case decision.KArrange:
		// The mid-resolution ordered-subset pick (Ruling J0): the engine's
		// one KArrange handler applies an answered library-arranging effect
		// (RearrangeTopOfLibrary, Ponder) -- the KArrange sibling of the
		// KModes case above, only ever asked mid-resolution.
		e.handleArrange(d, in)
	}
}

// handleChoose routes a choose answer to whichever flow asked it. The flows
// are data on the engine (never closures, so Clone copies them): the cast
// flow (Task 9), a miracle offer (Task 18), an "as this enters" choice
// (Task 12). A choose nobody is waiting for -- only reachable from a
// hand-built decision -- is dropped with a Note and priority resumes.
func (e *Engine) handleChoose(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	// A Station tap pick (rules/station.go) is a plain priority-action ask,
	// never a cast/cleanup flow and never a mid-resolution resume: route it
	// first, by the flow marker the ask set.
	if e.choosing == chooseStation {
		e.choosing = chooseNone
		e.handleStation(e.stationing, chosen)
		return
	}
	// Every KChoose carrying a resume point is a mid-resolution effect ask,
	// regardless of its ResumeKind (search, dig, imprint, untap selection,
	// reveal-optional, defined-library-optional, ward windows, hand_move,
	// sacrifice, roll, and future siblings). Dispatch by role rather than an
	// allowlist: the asking effect already recorded the exact SA and answer
	// interpretation in e.resume, while cast/cleanup flows never do (the
	// kinds that bypass this check -- madness, optional triggers, their
	// Cost$ windows -- are answered through KTriggerOptional and the
	// chooseTriggeredCost/chooseCumulative arms, never KChoose). This is the
	// structural guard against silently dropping the next KChoose-based
	// primitive merely because its string was not added here. An empty chosen
	// slice is the legitimate "fail to find" / Optional-decline answer (for
	// "roll" a malformed empty answer falls back to the first die inside
	// the effect).
	if e.resume != nil {
		rp := e.resume
		e.resume = nil
		e.resumeResolution(rp, chosen)
		return
	}
	switch e.choosing {
	case chooseCast:
		e.castAnswer(d, chosen)
		// A mana ability selection or Produced$ Any colour choice installed
		// its own decision; only a fully resolved singleton may continue.
		if e.pending != nil || e.choosing == chooseMana || e.choosing == chooseManaColor || e.choosing == chooseManaDiscard || e.choosing == chooseManaExile {
			return
		}
		e.continueCast()
		// Task 18: a Miracle cast originated inside the trigger drain (castMiracle
		// set drainAwaitsTarget when it paused on this X/Delve/Sac ask). Once the
		// cast commits -- continueCast reached commitCast and asked nothing more,
		// so no decision is pending -- the drain must resume through the SAME
		// continuation Task 7's target path uses, not hand caster priority: a
		// later, unrelated trigger in the same batch is still placed before any
		// player acts, and re-entering a fresh step would run a second state-based
		// pass. If commitCast instead asked a target, handleTarget honours the
		// still-true flag itself. A non-miracle cast never sets drainAwaitsTarget,
		// so this branch is inert for an ordinary X/Delve/Sac cast.
		if e.drainAwaitsTarget && e.Pending() == nil {
			e.drainAwaitsTarget = false
			e.resumeTriggerDrain()
			return
		}
	case chooseCommanderColor:
		// The CR 903.4b pregame colour choice was answered (rules/
		// commander_color.go): record it on the commander object as the same
		// Choose "color" event an as-enters ask uses, then advance the round.
		e.answerCommanderColor(d, chosen)
	case chooseUntap:
		if e.untapChoiceObj == 0 || len(chosen) != 1 {
			e.choosing = chooseNone
			e.untapChoiceObj = 0
			e.untapResume = nil
			return
		}
		id := e.untapChoiceObj
		next := 0
		if e.untapResume != nil {
			next = e.untapResume.next
		}
		keep := chosen[0].Index == 1
		e.emit(events.Event{Kind: events.Choose, Obj: id, Counter: "untap",
			Text: map[bool]string{true: "keep", false: "untap"}[keep]})
		e.choosing = chooseNone
		e.untapChoiceObj = 0
		if !keep {
			e.untapTurnPermanent(id)
			if e.pending != nil {
				return
			}
		}
		e.untapResume = nil
		if e.finishUntapStep(next) {
			e.finishEnteredStep()
		}
	case chooseRiot:
		// Riot is an as-enters replacement for every MoveZone path, including
		// reanimation and blink that never create pendingCast. Record the
		// choice first, then re-emit the parked entry; Apply consumes it on
		// battlefield entry.
		if e.riotMove == nil || len(chosen) != 1 {
			e.riotMove = nil
			e.choosing = chooseNone
			e.emit(events.Event{Kind: events.Note, Player: in.Player, Text: "Riot answered with no entry pending"})
			return
		}
		choice := "haste"
		if chosen[0].Index == 0 {
			choice = "counter"
		}
		e.emit(events.Event{Kind: events.Choose, Obj: e.riotMove.Obj, Counter: "riot", Text: choice})
		move := *e.riotMove
		e.riotMove = nil
		e.choosing = chooseNone
		e.emit(move)
	case chooseUnleash:
		// kw:Unleash (CR 702.86, rules/unleash.go) is an as-enters replacement
		// for every MoveZone path, the Riot arm's exact shape: record the
		// choice, then re-emit the parked entry; Apply consumes it on
		// battlefield entry.
		if e.unleashMove == nil || len(chosen) != 1 {
			e.unleashMove = nil
			e.choosing = chooseNone
			e.emit(events.Event{Kind: events.Note, Player: in.Player, Text: "Unleash answered with no entry pending"})
			return
		}
		choice := "plain"
		if chosen[0].Index == 0 {
			choice = "counter"
		}
		e.emit(events.Event{Kind: events.Choose, Obj: e.unleashMove.Obj, Counter: "unleash", Text: choice})
		move := *e.unleashMove
		e.unleashMove = nil
		e.choosing = chooseNone
		e.emit(move)
	case chooseAttached:
		if e.attachedChoice == nil || len(chosen) != 1 {
			e.attachedChoice = nil
			e.choosing = chooseNone
			return
		}
		ch := e.attachedChoice
		if ch.stage == 0 {
			e.emit(events.Event{Kind: events.Choose, Obj: ch.source, Counter: "name", Text: chosen[0].Label})
			e.askAttachedType()
			return
		}
		e.emit(events.Event{Kind: events.Choose, Obj: ch.source, Counter: "type", Text: chosen[0].Label})
		move := ch.move
		e.attachedChoice = nil
		e.choosing = chooseNone
		e.emitAttachedMove(move)
	case chooseSiege:
		// CR 310.10: the Battle Siege protector choice was answered. Record
		// the chosen opponent through a Choose "protector" event (so the
		// protector is replay-derived, never a direct field write), then
		// re-emit the parked entry -- Apply consumes the choice on entry.
		if e.siegeMove == nil || len(chosen) != 1 {
			e.siegeMove = nil
			e.choosing = chooseNone
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "Siege protector answered with no entry pending"})
			return
		}
		move := *e.siegeMove
		e.emit(events.Event{Kind: events.Choose, Obj: move.Obj,
			Counter: "protector", Player: chosen[0].Player})
		e.siegeMove = nil
		e.choosing = chooseNone
		e.emit(move)
	case chooseOpening:
		e.handleOpening(d, in)
	case chooseSuspendCast:
		// CR 702.62a: the may-cast offer on a suspended card's last TIME
		// counter was answered. suspendCastAnswer either enters the ordinary
		// cast flow (a yes) or leaves the card in exile and offers the next
		// suspended cast, if any (a decline). There is no trigger drain to
		// resume: the offer comes from the turn structure, never from inside
		// one, so e.drainAwaitsTarget is necessarily false here.
		e.suspendCastAnswer(chosen)
	case chooseETB:
		// Task 12: an "as this enters" choice was answered. Record it on the
		// card (etbAnswer, via a Choose event), then continue the flow -- the
		// cast flow's next (or remaining) etb choice, then commitCast; for a
		// land, commitCast moves it onto the battlefield. The drain resume is
		// the same shape as the chooseCast case, for the same reason (a
		// miracle cast whose own card also carried an as-enters choice would
		// have paused here mid-drain).
		e.etbAnswer(d, chosen)
		e.continueCast()
		if e.drainAwaitsTarget && e.Pending() == nil {
			e.drainAwaitsTarget = false
			e.resumeTriggerDrain()
			return
		}
	case chooseCleanup:
		// Task D1 (CR 514.1): the cleanup-step discard decision was answered.
		// discardCleanup moves the chosen cards hand -> graveyard, runs the
		// CR 514.2 body, and advances the step -- the cleanup step's resume,
		// the mirror of the cast-flow resume cases above. There is no drain
		// to resume: a discard answer is never handed out from inside a
		// trigger drain (only the turn structure asks it, when no step is
		// mid-resolution), so e.drainAwaitsTarget is necessarily false here.
		e.discardCleanup(chosen)
	case chooseCumulative:
		// The cumulative-upkeep trigger is resolving and waiting in its
		// mana/payment window (rules/cumulative.go).
		e.cumulativeAnswer(chosen)
	case chooseTriggeredCost:
		// A Cost$ carried by a triggered effect (Mana Vault's pay-{4} untap)
		// is paid during resolution rather than being silently ignored.
		e.triggeredCostAnswer(chosen)
	case chooseTriggeredMandatory:
		// trigmand1: a `Cost$ Mandatory Sac<...>/Exile<...>` trigger body's
		// choice-bearing component was answered. The component's picks are
		// validated and settled; no pay/decline election is ever posed.
		e.triggeredMandatoryAnswer(chosen)
	case chooseEcho:
		// kw:Echo's pay-or-sacrifice election (rules/echo.go) was answered.
		e.echoAnswer(chosen)
	case chooseDamageDivision:
		// Task jj-cmb (F40): the combat damage step's controller
		// damage-division decision (CR 510.1c) was answered.
		// handleDamageDivision records the chosen split and either asks the
		// next undone division or deals the pass -- the combat damage step's
		// own resume. There is no trigger drain to resume (a division answer
		// is never handed out from inside one).
		e.handleDamageDivision(chosen)
	case chooseAsUnblockedElection:
		// asunblk1: the combat damage step's assign-as-unblocked election
		// (stat:AssignCombatDamageAsUnblocked, CR 509) was answered.
		// handleAsUnblockedElection records the accepted elections and then
		// asks the next combat ask or deals the pass. There is no trigger
		// drain to resume (an election answer is never handed out from
		// inside one).
		e.handleAsUnblockedElection(chosen)
	case chooseExert:
		// exert1: the declare-attackers step's exert election (CR 702.100a)
		// was answered. exertAnswer emits the decline's nothing or the
		// accepted exert's event, then advances the cursor to the next
		// offerable attacker or ends the election; the Advance loop resumes
		// the step's own priority round after the last ask. The queued rider
		// triggers (the static's Trigger$ body, walk checkExertTriggers) are
		// placed at that same priority round, after every attack trigger the
		// declaration itself queued -- the ordinary APNAP drain, no separate
		// resume needed here.
		e.exertAnswer(d, in)
	case chooseEnlist:
		// enlist1: the declare-attackers step's enlist election (CR 702.160a)
		// was answered. enlistAnswer taps the chosen creature, emits the
		// Enlist event and registers the +power/+0 pump, then advances the
		// cursor to the next offerable attacker or finishes the declaration
		// (finishAttackers), whose DeclareAttackers events queue the attack
		// triggers -- so the enlist marker is already folded when an
		// intervening-if reads enlistedThisCombat.
		e.enlistAnswer(d, in)
	case chooseAttackPay:
		// The declare-attackers attack-cost payment window.
		e.attackPayAnswer(d, in)
	case chooseBlockPay:
		// The declare-blockers CantBlockUnless payment window. Completion
		// emits the parked declaration and advances blockerRound.
		e.blockPayAnswer(d, in)
	case chooseMana:
		// Several individual mana abilities share one tap cost. A payment
		// window resumes its cast after the selected ability resolves; Ward's
		// mid-resolution payment window reopens instead. An ordinary
		// activation falls through to Advance's priority round.
		cast := e.answerManaActivation(chosen)
		if e.pending == nil && e.choosing != chooseManaColor && e.choosing != chooseManaDiscard && e.choosing != chooseManaExile {
			if e.wardMana != nil {
				e.continueWardMana()
			} else if cast {
				e.continueCast()
			}
		}
	case chooseManaDiscard:
		cast := e.answerManaDiscard(chosen)
		if e.pending == nil && e.choosing != chooseManaColor && e.choosing != chooseManaDiscard && e.choosing != chooseManaExile {
			if e.wardMana != nil {
				e.continueWardMana()
			} else if cast {
				e.continueCast()
			}
		}
	case chooseManaExile:
		cast := e.answerManaExile(chosen)
		if e.pending == nil && e.choosing != chooseManaColor && e.choosing != chooseManaDiscard && e.choosing != chooseManaExile {
			if e.wardMana != nil {
				e.continueWardMana()
			} else if cast {
				e.continueCast()
			}
		}
	case chooseUnlessCost:
		// A Sac/Discard component of an already-accepted UnlessCost$ needs
		// its payer's real choice before the suspended effect can resume.
		e.answerUnlessPayment(chosen)
	case chooseManaColor:
		// A CR 605.3b triggered mana ability may pose its own colour choice
		// after this one; the cast (or Ward's payment window) resumes only
		// once none is pending.
		cast := e.answerManaColor(chosen)
		if e.pending == nil && e.choosing != chooseManaColor {
			if e.wardMana != nil {
				e.continueWardMana()
			} else if cast {
				e.continueCast()
			}
		}
	// Tasks 12, 18 add their cases here; Task D1 adds chooseCleanup.
	default:
		e.emit(events.Event{Kind: events.Note, Player: in.Player, Text: "choose answered with no flow waiting"})
		e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
	}
}

// rotationBase returns the seat whose turn the ordinary rotation is currently
// ON: the holder of the most recent NORMAL turn -- a TurnChange that did not
// begin an extra turn. An extra turn's TurnChange is the one a cleanup step
// consumed a grant for: a -1 ExtraTurn event sits between the previous
// TurnChange and it (backward window below), so scanning backward and
// skipping every TurnChange whose backward window holds a consumption lands
// on the last normal holder. When the pending-extra queue drains, the next
// turn is this seat's successor (NextAlive) -- the extra turns were inserted
// after that seat's turn, never in place of the seats that follow it. No
// TurnChange at all cannot happen (turn 1 opens the log); the fallback names
// seat 0 for totality.
//
// The scan is over the log, never live state, so a replay re-derives the
// same base. Cost is one backward window per extra-turn cleanup -- a
// window is one turn's events -- and zero for every cleanup of a normal
// turn whose predecessor was also normal (the common case stops at the
// first TurnChange).
func (e *Engine) rotationBase() state.PlayerID {
	evs := e.L.Events
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind != events.TurnChange {
			continue
		}
		// Some focused rules fixtures deliberately seed Game.Active/Turn at a
		// mid-turn state without rewriting their genesis log. That live state
		// is authoritative: it is an ordinary turn unless an in-log consumption
		// says otherwise, so its successor is based on the live active seat.
		if evs[i].Player != e.G.Active || evs[i].Amount != e.G.Turn {
			return e.G.Active
		}
		// Backward window: (previous TurnChange, exclusive) .. (this one,
		// exclusive). A -1 consumption of THIS TurnChange's own seat in it
		// means THIS TurnChange began an extra turn (the consumption is
		// emitted immediately before beginTurn); lost-seat skips consume
		// several, all inside the window. The player check is what keeps a
		// SKIPPED grant's consumption -- same -1 form, no beginTurn, so the
		// next TurnChange is the ORDINARY rotation's, a different seat --
		// from classifying that ordinary turn as extra: without it the next
		// cleanup would base the rotation on the seat before the granted
		// seat and hand it its turn again. The one seat the check cannot
		// distinguish is a lone survivor whose own skipped grant is followed
		// by the ordinary rotation rotating back to itself -- unreachable in
		// a real 2+ seat game and not worth a new event kind.
		extra := false
		for j := i - 1; j >= 0; j-- {
			if evs[j].Kind == events.TurnChange {
				break
			}
			if evs[j].Kind == events.ExtraTurn && evs[j].Amount < 0 && evs[j].Player == evs[i].Player {
				extra = true
				break
			}
		}
		if !extra {
			return evs[i].Player
		}
	}
	return 0
}

// latestUnconsumedGrant returns the ExtraTurn grant event the NEXT
// consumption of seat's pending grant consumes: walking the log backward, a
// -1 consumption matches the most recent still-unconsumed +grant of the same
// seat (the same latest-first order the turn structure consumes in), so the
// first +grant reached with the running consumed-count at zero IS the grant
// whose rider (Final Fortune's ExtraTurnDelayedTrigger$/Execute$ pair,
// carried on the event's Obj/Counter, plus the trigger's Phase$ in IDs[0])
// must ride the consumption that takes its turn.
// A seat with no grant event left (never happens while its queue entry is
// pending; the fold guarantees the count) returns zeros -- the consumption
// then carries no rider and events.Apply registers nothing.
func (e *Engine) latestUnconsumedGrant(seat state.PlayerID) (state.ObjID, string, state.ObjID) {
	consumed := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind != events.ExtraTurn || ev.Player != seat {
			continue
		}
		if ev.Amount < 0 {
			consumed += int(-ev.Amount)
			continue
		}
		// One +Amount event records Amount individual grants. Earlier
		// consumptions may account for only its newest entries; otherwise this
		// event is the source of the next queue entry and its rider belongs to
		// that turn too.
		if consumed >= int(ev.Amount) {
			consumed -= int(ev.Amount)
			continue
		}
		phase := state.ObjID(state.StepEnd)
		if len(ev.IDs) > 0 {
			phase = ev.IDs[0]
		}
		return ev.Obj, ev.Counter, phase
	}
	return 0, "", 0
}
