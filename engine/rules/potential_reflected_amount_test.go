package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const reflAmtDruid = "Name:Reflect Druid\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:0/2\n" +
	"A:AB$ ManaReflected | Cost$ T | ColorOrType$ Type | Valid$ Land.YouCtrl | Amount$ ReflAmt | ReflectProperty$ Produce\n" +
	"SVar:Cnt:Count$Valid Card.Self+counters_GE1_P1P1\nSVar:ReflAmt:Count$Compare Cnt GE1.3.1\nOracle:x\n"

func reflAmtLand(name, colour string) string {
	return "Name:" + name + "\nTypes:Basic Land " + name + "\nA:AB$ Mana | Cost$ T | Produced$ " + colour + " | SpellDescription$ Add {" + colour + "}.\nOracle:x\n"
}

// TestPotentialManaColoursAReflectedAmountSource pins CR 106.7 in the bound: a
// live reflected Amount$ druid (Incubation Druid's shape) prices each colour
// the lands "could produce", tapped or not, so a druid beside tapped Swamps
// bounds {B}; the same druid beside a Forest alone bounds no {B}.
func TestPotentialManaColoursAReflectedAmountSource(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	druid := onBoard(t, e, 0, reflAmtDruid)
	swamp := onBoard(t, e, 0, reflAmtLand("Swamp", "B"))
	e.G.Obj(druid).SummonSick = false
	e.G.Obj(swamp).Tapped = true
	if !e.G.Obj(swamp).Tapped || e.G.Obj(druid).Tapped {
		t.Fatal("precondition: the Swamp must be tapped and the druid untapped")
	}
	if got := e.PotentialMana(0)[state.MB]; got <= 0 {
		t.Fatalf("PotentialMana {B} = %d beside a tapped Swamp, want the reflected colour priced", got)
	}

	f := layerEngine(t)
	druid = onBoard(t, f, 0, reflAmtDruid)
	forest := onBoard(t, f, 0, reflAmtLand("Forest", "G"))
	f.G.Obj(druid).SummonSick = false
	f.G.Obj(forest).Tapped = true
	if got := f.PotentialMana(0)[state.MB]; got != 0 {
		t.Fatalf("PotentialMana {B} = %d with no Swamp, want 0", got)
	}
	if got := f.PotentialMana(0)[state.MG]; got <= 0 {
		t.Fatalf("PotentialMana {G} = %d beside a tapped Forest, want the reflected colour priced", got)
	}
}
