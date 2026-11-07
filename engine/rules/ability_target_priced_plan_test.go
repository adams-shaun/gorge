package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Cloud, Planet's Champion's shape: equip abilities that target this
// creature cost {2} less (a ValidTarget$ ReduceCost on an activation).
const atpCloud = "Name:Target Discount Hero\nManaCost:3 R W\nTypes:Legendary Creature Human Soldier\nPT:4/4\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2 | Description$ x\nOracle:x\n"

const atpArms = "Name:Heavy Arms\nManaCost:2 W\nTypes:Artifact Equipment\nK:Equip:4\nOracle:x\n"

const atpBear = "Name:Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// TestAbilityPlanWitnessesTargetPricedEquip: with two untapped lands and an
// empty pool, Equip {4} is payable only by announcing the discounted target
// (CR 601.2f prices the activation after its target is chosen). The ability
// planner used to witness only the untargeted {4}, so the autopay offer
// withheld an activation the offer gate itself proved payable. It now plans
// the target-priced {2}: the equip is offered, and activating it at the
// discounted target taps both lands without a fallback.
func TestAbilityPlanWitnessesTargetPricedEquip(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9710, ppFiller)
	toMain1(t, e)
	e.paidManaCensus = true
	hero := onBoardReady(t, e, 0, atpCloud)
	bear := onBoardReady(t, e, 0, atpBear)
	arms := onBoardReady(t, e, 0, atpArms)
	lands := []state.ObjID{onBoardReady(t, e, 0, apaForest), onBoardReady(t, e, 0, apaForest)}

	// Preconditions: the untargeted price is the printed {4}, more than the
	// two lands make, and the floating pool is empty.
	idx := -1
	for i := range e.G.Obj(arms).Face().Abilities {
		if e.G.Obj(arms).Face().Abilities[i].Kind == "AB" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("precondition: the equipment has no Equip ability")
	}
	ab := e.G.Obj(arms).Face().Abilities[idx]
	if c := e.offerCostFor(0, arms, e.abilityCompiledCost(ab).Cost, abilityScope(ab)); c.Generic != 4 {
		t.Fatalf("precondition: untargeted equip price = %+v, want generic 4", c)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatal("precondition: the pool is not empty")
	}

	out := e.planAbilityPayment(0, arms, idx)
	if out.Plan == nil || out.Reason != "" || len(out.Plan.Activations) != 2 {
		t.Fatalf("plan = %+v, want a two-land witness of the target-priced {2}", out)
	}

	d := ppPriority(t, e)
	opt := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == arms {
			opt = o.Index
		}
	}
	if opt < 0 {
		t.Fatalf("the target-priced equip was not offered: %+v", d.Options)
	}
	submitChoices(t, e, opt)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending after activating = %+v, want the equip's target ask", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == hero {
			pick = o.Index
		}
		if o.Obj == bear {
			t.Fatalf("the unaffordable full-price target was offered: %+v", d.Options)
		}
	}
	if pick < 0 {
		t.Fatalf("the discounted target is missing: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	for i := 0; i < 8; i++ {
		d = e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		if d.PaymentFallback != nil {
			t.Fatalf("payment fell back %+v at %q", *d.PaymentFallback, d.Prompt)
		}
		var least []int
		for _, o := range d.Options[:d.Min] {
			least = append(least, o.Index)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: least}); err != nil {
			t.Fatalf("answer %q: %v", d.Prompt, err)
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the equip activation did not reach the stack")
	}
	for _, l := range lands {
		if !e.G.Obj(l).Tapped {
			t.Fatalf("land %d untapped: the {2} was not paid from the lands", l)
		}
	}
}
