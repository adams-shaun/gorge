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
	e.emit(events.Event{Kind: events.StepChange, Step: s})
	// Mana pools empty as each step ends (CR 500.4).
	for i := range e.G.Players {
		if e.G.Players[i].Pool.Total() > 0 {
			e.emit(events.Event{Kind: events.ManaClear, Player: state.PlayerID(i)})
		}
	}
	if s == state.StepEndCombat || s == state.StepCleanup {
		// Ruling T21-e: routed through an event (events.EndCombatReset), not
		// a direct field write -- a log-only replay must learn that combat
		// ended and IsAttacking/BlockedBy were cleared, not just observe it
		// as a fait accompli baked into a live Engine's memory.
		e.emit(events.Event{Kind: events.EndCombatReset})
	}
}

// step performs the smallest unit of automatic engine work.
func (e *Engine) step() {
	e.checkStateBased()
	if e.G.Over {
		return
	}
	switch e.G.Step {
	case state.StepDeclareAttackers:
		e.askAttackers()
	case state.StepDeclareBlockers:
		e.askBlockers()
	case state.StepCombatDamage:
		e.dealCombatDamage()
		e.setStep(state.StepEndCombat)
	default:
		e.priorityRound()
	}
}

func (e *Engine) priorityRound() {
	// Nobody receives priority during untap or cleanup.
	if e.G.Step == state.StepUntap || e.G.Step == state.StepCleanup {
		if e.G.Step == state.StepCleanup {
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
			e.cleanupStep()
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
	if e.G.Step == state.StepCleanup {
		// beginTurn resets the pass count along with the new holder.
		e.beginTurn(e.G.NextAlive(e.G.Active))
		return
	}
	e.setStep(e.G.Step + 1)
	if e.G.Step == state.StepDraw && e.G.Turn > 1 && !e.G.Players[e.G.Active].Lost {
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
		// e.G.Turn > 1 keeps CR 103.8a: the game's very first turn skips its
		// draw. !Lost keeps an eliminated active player from drawing.
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
	case decision.KChoose:
		e.handleChoose(d, in)
	}
}

// handleChoose routes a choose answer to whichever flow asked it. The flows
// are data on the engine (never closures, so Clone copies them): the cast
// flow (Task 9), a miracle offer (Task 18), an "as this enters" choice
// (Task 12). A choose nobody is waiting for -- only reachable from a
// hand-built decision -- is dropped with a Note and priority resumes.
func (e *Engine) handleChoose(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	switch e.choosing {
	// Tasks 9, 12 and 18 add their cases here.
	default:
		e.emit(events.Event{Kind: events.Note, Player: in.Player, Text: "choose answered with no flow waiting"})
		e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
	}
	_ = chosen
}
