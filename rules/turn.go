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
	e.G.Priority = active
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
		for i := range e.G.Objs {
			e.G.Objs[i].IsAttacking = false
			e.G.Objs[i].BlockedBy = nil
		}
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
		e.advanceStep()
		return
	}
	// The active player's draw happens once, before anyone gets priority.
	if e.G.Step == state.StepDraw && e.G.Turn > 1 && e.G.Passes == 0 && e.G.Priority == e.G.Active {
		e.drawCard(e.G.Active)
	}
	if e.G.Players[e.G.Priority].Lost {
		e.G.Priority = e.G.NextAlive(e.G.Priority)
	}
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority})
	e.askPriority(e.G.Priority)
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
	e.G.Passes = 0
	if e.G.Step == state.StepCleanup {
		e.beginTurn(e.G.NextAlive(e.G.Active))
		return
	}
	e.setStep(e.G.Step + 1)
	e.G.Priority = e.G.Active
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
