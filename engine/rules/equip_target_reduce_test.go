package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

const maulerSrc = "Name:Dwarven Mauler\nManaCost:2 R\nTypes:Creature Dwarf Berserker\nPT:2/1\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2\n" +
	"Oracle:Equip abilities you activate that target this creature cost {2} less to activate.\n"

const mattockEquipSrc = "Name:Dwarven Mattock\nManaCost:2\nTypes:Artifact Equipment\n" +
	"K:Equip:3\nOracle:Equip {3}\n"

// TestEquipTargetReductionAutoPaidFromLands: a target-gated equip reduction
// (Dwarven Mauler, CR 601.2f) makes Equip {3} aimed at the Mauler cost {1}, so
// two untapped lands and an empty pool offer it (the autopay witness is
// planned over the cheapest legal target's price), and choosing it works.
func TestEquipTargetReductionAutoPaidFromLands(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 71, ppFiller, mattockEquipSrc, maulerSrc)
	e.paidManaCensus = true
	mat := putCreature(t, e, 0, mattockEquipSrc)
	_ = putCreature(t, e, 0, maulerSrc)
	onBoard(t, e, 0, lrIsland)
	onBoard(t, e, 0, lrIsland)
	d := ppPriority(t, e)
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", e.G.Players[0].Pool)
	}
	opt, ok := autopayAbilityOffer(d, mat)
	if !ok {
		t.Fatalf("Equip {3} targeting the Mauler (cost {1}) not offered over two lands: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatal(err)
	}
	if td := e.Pending(); td == nil || td.Kind != decision.KTarget {
		t.Fatalf("want the equip target ask, got %+v", e.Pending())
	}
}
