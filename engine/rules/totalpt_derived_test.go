package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTotalPTPredicateReadsLayerSevenCPump pins the rules seam for Forge's
// totalPT property: matchesSpec must bind the layer-derived P/T for a totalPT
// spec, so a layer-7c pump moves the answer (a 2/3 pumped to 3/4 is total 7,
// not Cut Down's printed 5).
func TestTotalPTPredicateReadsLayerSevenCPump(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Pump Subject\nTypes:Creature\nPT:2/3\nOracle:x\n")
	if !computeSpecReadsDerived("Creature.totalPT_LE5") {
		t.Fatal("precondition: a totalPT spec must be classified as reading derived P/T")
	}
	if !e.matchesSpec("Creature.totalPT_LE5", bear, effects.SpecContext{}) {
		t.Fatal("precondition: the unpumped 2/3 (total 5) must match totalPT_LE5")
	}
	onBoard(t, e, 0, "Name:Anthem\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.Other | AddPower$ 1 | AddToughness$ 1 | Description$ pump\nOracle:x\n")
	if d := e.Derived(bear); d.Power != 3 || d.Toughness != 4 {
		t.Fatalf("precondition: pumped creature is %d/%d, want 3/4", d.Power, d.Toughness)
	}
	if e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatal("precondition: candidate must be on the battlefield")
	}
	if e.matchesSpec("Creature.totalPT_LE5", bear, effects.SpecContext{}) {
		t.Error("a layer-7c-pumped 3/4 (total 7) must not match totalPT_LE5")
	}
	if !e.matchesSpec("Creature.totalPT_GE7", bear, effects.SpecContext{}) {
		t.Error("a layer-7c-pumped 3/4 (total 7) must match totalPT_GE7")
	}
}
