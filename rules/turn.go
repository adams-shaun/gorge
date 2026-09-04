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
	e.putTriggersOnStack()
	holder := e.G.Priority
	if e.G.Players[holder].Lost {
		holder = e.G.NextAlive(holder)
	}
	e.emit(events.Event{Kind: events.Priority, Player: holder, Amount: e.G.Passes})
	e.askPriority(holder)
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
	}
}
