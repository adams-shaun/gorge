package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// CR 613.4c: Mogg Jailer's UnlessDefender$ controlsCreature.untapped+powerLE2
// reads the defender's creatures' DERIVED power. A 1/1 that wears a +2/+2 pump
// (Gaea's Might) has power 3 and no longer holds the Jailer back; the printed
// power 1 once did (tail-33 autopay pair 63).
func TestMoggJailerReleasedByPumpedDefender(t *testing.T) {
	t.Parallel()
	e := attackBlockedRegressionEngine(t)
	jailer := onBoardCard(t, e, 0, corpusCard(t, "Mogg Jailer"))
	const src = "Name:Test Bear\nTypes:Creature Bear\nPT:1/1\n" +
		"SVar:Boost:DB$ Pump | Defined$ Self | NumAtt$ +2 | NumDef$ +2\nOracle:x\n"
	bear := onBoard(t, e, 1, src)
	if !e.attackBlocked(jailer, 1, 0) {
		t.Fatal("precondition: Mogg Jailer attacked a defender with an untapped 1/1")
	}
	body := cards.ResolveSVar(e.G.Obj(bear).Card.Faces[0].SVars, "Boost")
	if body == nil || body.API != "Pump" {
		t.Fatalf("precondition: fixture Pump body missing: %+v", body)
	}
	effects.Resolve(e, &effects.Ctx{Source: bear, Controller: 1}, body)
	if p := e.Power(bear); p != 3 {
		t.Fatalf("precondition: pumped defender power = %d, want 3", p)
	}
	if e.attackBlocked(jailer, 1, 0) {
		t.Fatal("Mogg Jailer stayed blocked although the defender's only creature has derived power 3")
	}
}
