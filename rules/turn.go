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
		// Ruling T21-a: routed through an event (events.EndCombatReset), not
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
			// run before advanceStep, not after: advanceStep's own Passes
			// reset (via beginTurn) is load-bearing for the draw-step trigger
			// gate and is left untouched either way, but cleanup logically
			// belongs to the step being left, not the one being entered.
			e.cleanupStep()
		}
		e.advanceStep()
		return
	}
	// The active player's draw happens once, before anyone gets priority.
	// An eliminated active player draws nothing: unreachable today (drawing
	// from an empty library is the only elimination source, and that seat
	// would already be Lost, not still due a draw), but state-based actions
	// will add other ways to lose without touching the draw step.
	if e.G.Step == state.StepDraw && e.G.Turn > 1 && e.G.Passes == 0 &&
		e.G.Priority == e.G.Active && !e.G.Players[e.G.Active].Lost {
		e.drawCard(e.G.Active)
	}
	// The draw above can eliminate the active player and end the game (an
	// empty-library draw is a loss). A finished game must not hand out a
	// fresh priority decision.
	if e.G.Over {
		return
	}
	// CR 117.5: state-based actions and triggered abilities are handled
	// before any player receives priority.
	//
	// Task 27: putTriggersOnStack can now ASK -- a controller with two or
	// more simultaneous triggers orders them, and an optional trigger's
	// decider says yes or no -- so it reports whether it finished. Returning
	// here on a true is load-bearing twice over. It stops askPriority from
	// immediately overwriting e.pending with a second, unrelated decision;
	// and it is why the drain resumes through resumeTriggerDrain below rather
	// than by re-entering this function, which would run the draw step's draw
	// a second time (nothing between the draw above and the priority emit
	// below changes Passes or Priority, so this function is not idempotent
	// and must not be re-entered mid-round).
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
	}
}
