package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 601.2f / 602.2b: an equip cost reduction gated on the equip's target
// ("Equip abilities you activate that target this creature cost {2} less",
// Dwarven Mauler) must be priced by the AUTOPAY planner, not only by the
// floating-pool offer: with no mana floating and two untapped Mountains the
// {3} equip is payable at {1}, so it must be offered. Before the fix the offer
// gate priced the target-gated reduction but the planner planned the printed
// {3}, found no witness, and withheld the activation.
const (
	equipReduceAutopayMauler = "Name:Autopay Mauler\nManaCost:R\nTypes:Creature Dwarf Warrior\nPT:2/1\n" +
		"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2 | Description$ x\nOracle:x\n"
	equipReduceAutopayBlade = "Name:Autopay Blade\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:3\nOracle:x\n"
	equipReduceAutopayLand  = "Name:Autopay Mountain\nTypes:Land Mountain\nOracle:x\n"
)

func TestTargetGatedEquipReductionOfferedFromAutopay(t *testing.T) {
	t.Parallel()
	e, cfg, blade := newFixtureDeck(t, 741, equipReduceAutopayBlade, equipReduceAutopayMauler, equipReduceAutopayLand, equipReduceAutopayLand)
	e.emit(events.Event{Kind: events.MoveZone, Obj: blade, From: state.ZHand, To: state.ZBattlefield})
	mauler := moveSeeded(t, e, 0, equipReduceAutopayMauler, state.ZBattlefield)
	moveSeeded(t, e, 0, equipReduceAutopayLand, state.ZBattlefield)
	moveSeeded(t, e, 0, equipReduceAutopayLand, state.ZBattlefield)
	addMana(t, e, 0, "")
	// Preconditions: the printed equip really is {3}, the mauler is the only
	// creature, and payment must come from two untapped lands (nothing
	// floating) -- exactly the shape the pool-only offer gate cannot price.
	o := e.G.Obj(blade)
	if o == nil || o.Face() == nil {
		t.Fatal("fixture blade missing")
	}
	pa, ok := o.PileAbilityAt(0)
	if !ok || pa.SA == nil {
		t.Fatal("fixture blade has no equip ability at index 0")
	}
	if got := e.abilityCompiledCost(pa.SA).Cost.Generic; got != 3 {
		t.Fatalf("printed equip generic = %d, want 3", got)
	}
	if got := e.Power(mauler); got != 2 {
		t.Fatalf("fixture mauler power = %d, want 2", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool before the offer = %d, want 0 (the planner, not floating mana, must fund it)", got)
	}
	if !e.hasUntappedManaSource(0) {
		t.Fatal("no untapped source: the autopay path would not be exercised")
	}
	opt, ok := findAbilityOption(e, blade, 0)
	if !ok {
		t.Fatalf("target-gated equip ({3} reduced to {1}) not offered from two untapped Mountains: the planner planned the unreduced cost")
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, mauler)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(blade).AttachedTo; got != mauler {
		t.Fatalf("blade attached to %d, want the mauler %d", got, mauler)
	}
	// {1} of the two Mountains is tapped; the second stays untapped. A
	// planner that tapped the printed {3} could not have paid it at all, but
	// over-tapping would show as both tapped.
	untapped := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id == blade || id == mauler {
			continue
		}
		ob := e.G.Obj(id)
		if ob != nil && ob.Face() != nil && ob.Face().Name == "Autopay Mountain" && !ob.Tapped {
			untapped++
		}
	}
	if untapped != 1 {
		t.Fatalf("untapped Mountains after the {1} equip = %d, want 1", untapped)
	}
	replayCheck(t, e, cfg)
}
