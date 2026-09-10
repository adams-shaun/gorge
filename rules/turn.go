package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func (e *Engine) beginTurn(active state.PlayerID) {
	e.emit(events.Event{Kind: events.TurnChange, Player: active, Amount: e.G.Turn + 1})
	e.setStep(state.StepUntap)
	for _, id := range e.G.Zone(state.ZBattlefield, active) {
		if e.G.Obj(id).Tapped {
			e.emit(events.Event{Kind: events.Untap, Obj: id})
		}
	}
	e.setStep(state.StepUpkeep)
	// Start of turn resets the pass count along with the holder.
	e.emit(events.Event{Kind: events.Priority, Player: active})
}

func (e *Engine) setStep(s state.Step) {
	leaving := e.G.Step
	e.emit(events.Event{Kind: events.StepChange, Step: s})
	// Mana pools empty as each step ends (CR 500.4).
	for i := range e.G.Players {
		if e.G.Players[i].Pool.Total() > 0 {
			e.emit(events.Event{Kind: events.ManaClear, Player: state.PlayerID(i)})
		}
	}
	if leaving == state.StepEndCombat && s != leaving {
		// CR 511.3 removes creatures and planeswalkers from combat as the end
		// of combat step ends, not when it begins. Keeping the leaving-step
		// boundary here covers every transition made through setStep exactly
		// once; the former cleanup safety net would emit a duplicate reset.
		// Ruling T21-e keeps the reset event-sourced so a log-only replay also
		// learns that IsAttacking and BlockedBy were cleared.
		e.emit(events.Event{Kind: events.EndCombatReset})
	}
}

// step performs the smallest unit of automatic engine work.
func (e *Engine) step() {
	e.checkStateBased()
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
	// Nobody receives priority during untap or cleanup.
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
			// emits the discard moves, runs the 514.2 body, and then advances
			// the step itself.
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
			e.cleanupStep()
			if e.pending != nil {
				return
			}
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
			e.G.Turn, e.G.Step, e.G.Players[p].Name),
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
		// beginTurn resets the pass count along with the new holder.
		e.beginTurn(e.G.NextAlive(e.G.Active))
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
	e.setStep(e.G.Step + 1)
	if e.G.Step == state.StepDraw && (len(e.G.Players) != 2 || e.G.Turn > 1) && !e.G.Players[e.G.Active].Lost {
		// CR 504.1: the draw step's draw is a TURN-BASED ACTION -- it happens
		// once, automatically, at the beginning of the step, before any
		// player receives priority, full stop. It is not conditioned on
		// priority state in any way.
		//
		// Ruling T23-x: before this, the draw lived in priorityRound, gated
		// on `Passes == 0 && Priority == Active` -- a PROXY for "the step
		// just began" that Task 23's own test author measured is also
		// exactly the state resolveTop's callers restore after every
		// resolution (CR 117.3b -- Priority{Player: e.G.Active, Amount: 0};
		// see the T14-e comments in stack.go / legal.go). So a mandatory
		// "whenever you draw a card" trigger that resolved during the draw
		// step made the proxy true again, drew a SECOND card, queued a
		// second trigger, and so on until the library ran out: one seat
		// drawing 20 cards inside what the log still called one step, the
		// other seat never getting a turn. Keying the draw on the step
		// being ENTERED, instead of on ambient Passes/Priority state that
		// anything resolving later in the step can also produce, makes it
		// run exactly once no matter what resolves afterward.
		//
		// Ruling F45: CR 103.8a skips the starting player's first draw only
		// in a two-player game. Multiplayer free-for-all games take that draw
		// normally (CR 800.7). len(e.G.Players) is the constructed seat count;
		// eliminated players remain in the slice, so the rule cannot change as
		// players lose. e.G.Turn > 1 preserves the draw on every later turn.
		// !Lost keeps an eliminated active player from drawing.
		//
		// Ruling T28-b (fix round 1): this guard is REACHABLE in ordinary
		// play, not a defensive leftover -- an earlier draft of this comment
		// called it "unreachable today" on the theory that an empty-library
		// draw was the only way to become Lost before this point, and Task
		// 22 already falsified that: any state-based action can eliminate
		// the active player during their OWN turn, before their OWN draw
		// step, for a reason that has nothing to do with drawing at all (CR
		// 704.5a life loss is the common case). Measured: an upkeep
		// self-drain (`Mode$ Phase | Phase$ Upkeep`) trigger (this repo's
		// own drainerSrc fuzz fixture) eliminates its controller during
		// that seat's turn-2 upkeep with their library still full, and turn
		// 2's draw step is then entered with the eliminated seat still
		// Active. The turn structure does not skip steps for an eliminated
		// active player, only priority -- so
		// their draw step is still entered, and this is what stops it from
		// drawing on their behalf. A reader who trusts "unreachable" here is
		// invited to delete this guard, and deleting it is exactly the
		// mutant that draws for an eliminated player.
		e.drawCard(e.G.Active)
		// The draw above runs checkStateBased (drawCard's own tail): an
		// empty-library draw is itself a loss (CR 704.5c), and that can end
		// the game outright. A finished game must not emit a further
		// Priority event or hand out a decision (mirrors priorityRound's own
		// pre-Task-27 "if e.G.Over { return }" after a state-changing call).
		if e.G.Over {
			return
		}
	}
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
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
	// A hidden-library ChangeZone uses KChoose's ordinary ordered subset wire
	// shape, but it is a mid-resolution effect ask rather than one of the cast/
	// cleanup flows tracked by e.choosing. Resume it before dispatching those
	// flows; an empty chosen slice is the legitimate "fail to find" answer.
	if e.resume != nil && e.resume.kind == "search" {
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
		if e.choosing == chooseMana || e.choosing == chooseManaColor || e.choosing == chooseManaDiscard {
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
	case chooseDamageDivision:
		// Task jj-cmb (F40): the combat damage step's controller
		// damage-division decision (CR 510.1c) was answered.
		// handleDamageDivision records the chosen split and either asks the
		// next undone division or deals the pass -- the combat damage step's
		// own resume. There is no trigger drain to resume (a division answer
		// is never handed out from inside one).
		e.handleDamageDivision(chosen)
	case chooseMana:
		// Several individual mana abilities share one tap cost. A payment
		// window resumes its cast after the selected ability resolves; an
		// ordinary activation falls through to Advance's priority round.
		if e.answerManaActivation(chosen) && e.choosing != chooseManaColor && e.choosing != chooseManaDiscard {
			e.continueCast()
		}
	case chooseManaDiscard:
		if e.answerManaDiscard(chosen) && e.choosing != chooseManaColor && e.choosing != chooseManaDiscard {
			e.continueCast()
		}
	case chooseManaColor:
		if e.answerManaColor(chosen) {
			e.continueCast()
		}
	// Tasks 12, 18 add their cases here; Task D1 adds chooseCleanup.
	default:
		e.emit(events.Event{Kind: events.Note, Player: in.Player, Text: "choose answered with no flow waiting"})
		e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
	}
}
