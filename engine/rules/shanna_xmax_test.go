package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestShannaTriggerXCappedByLifeGained pins XMax$ on a triggered "you may pay
// {X}" ask: Shanna, Purifying Blade's X "can't be greater than the amount of
// life you gained this turn", so after gaining 2 life the announced X options
// are exactly 0..2 even with five sources of mana available.
func TestShannaTriggerXCappedByLifeGained(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Shanna, Purifying Blade")
	searchMoveByName(t, e, "Shanna, Purifying Blade", state.ZBattlefield)
	for i := 0; i < 5; i++ {
		xFoldLand(t, e, "Test Forest "+string(rune('A'+i)), "G")
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if got := e.G.Players[0].LifeGainedThisTurn; got != 2 {
		t.Fatalf("precondition: life gained this turn = %d, want 2", got)
	}

	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	d := passUntilNonPriority(t, e, 40)
	if d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("expected the choose-X announcement ask, got %+v", d)
	}
	var got []int
	for _, o := range d.Options {
		got = append(got, o.Amount)
	}
	if len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("X options = %v, want [0 1 2] (capped by life gained this turn)", got)
	}
}
