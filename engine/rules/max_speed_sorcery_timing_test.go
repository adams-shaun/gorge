package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestMaxSpeedGrantKeepsSorceryTiming pins CR 602.5d on a granted ability: at
// max speed it is offered in the controller's main phase with an empty stack
// and nowhere else.
func TestMaxSpeedGrantKeepsSorceryTiming(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	id := onBoard(t, e, 0, "Name:Racer\nTypes:Artifact\n"+
		"S:Mode$ Continuous | AffectedDefined$ Self | Condition$ MaxSpeed | AddAbility$ ABGrow | Description$ At full speed it grows.\n"+
		"SVar:ABGrow:AB$ PutCounter | Cost$ T | ValidTgts$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | SorcerySpeed$ True | SpellDescription$ Grow one.\n"+
		"Oracle:x\n")
	onBoard(t, e, 0, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	setSpeed(t, e, maxSpeed)
	offered := func() bool {
		for _, o := range e.Pending().Options {
			if o.Kind == "granted" && o.Obj == id {
				return true
			}
		}
		return false
	}
	e.priorityRound()
	if e.G.Step != state.StepMain1 || !offered() {
		t.Fatalf("not offered in %v", e.G.Step)
	}
	for e.G.Step == state.StepMain1 {
		passOnceP(t, e)
	}
	if e.Pending().Player == 0 && offered() {
		t.Fatalf("offered in %v", e.G.Step)
	}
}
