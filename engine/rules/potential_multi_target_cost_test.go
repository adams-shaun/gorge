package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// This Town Ain't Big Enough: {4}{U}, "This spell costs {3} less to cast if
// it targets a permanent you control", "Return up to two target nonland
// permanents to their owners' hands." Its target declaration is TargetMin$ 0
// TargetMax$ 2, and ONE own permanent in the announced set is enough for the
// reduction. CR 601.2f prices the spell for the targets it will have, so the
// cheapest legal set (one own permanent, or two targets one of which is own)
// costs {1}{U}; the potential planner must witness that price, not the
// printed {4}{U}, or the cast is never offered (kfuzz std-otj pair 1576).
const ttabeSrc = "Name:This Town Ain't Big Enough\nManaCost:4 U\nTypes:Instant\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ 3 | ValidTarget$ Permanent.YouCtrl | EffectZone$ All | Description$ This spell costs {3} less to cast if it targets a permanent you control.\n" +
	"A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Hand | TgtPrompt$ Choose target nonland permanent | ValidTgts$ Permanent.nonLand | TargetMin$ 0 | TargetMax$ 2 | SpellDescription$ Return up to two target nonland permanents to their owners' hands.\n" +
	"Oracle:x\n"

func ttabeIsland() string { return "Name:TTABE Island\nTypes:Basic Land Island\nOracle:x\n" }

// TestPotentialMultiTargetCostPricesCheapestSet pins the potential planner's
// price for a target-dependent reduction on a spell that may choose several
// targets. The board holds exactly two mana ({U}{U} from two Islands), an own
// nonland permanent and an opponent's creature; the printed {4}{U} is
// unpayable, the {1}{U} price with the own permanent in the target set is
// not.
func TestPotentialMultiTargetCostPricesCheapestSet(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, ttabeSrc))
	spell := e.G.Zone(state.ZHand, 0)[0]
	onBoard(t, e, 0, ttabeIsland())
	onBoard(t, e, 0, ttabeIsland())
	own := onBoard(t, e, 0, "Name:TTABE Relic\nTypes:Artifact\nOracle:x\n")
	opp := onBoard(t, e, 1, "Name:TTABE Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if o := e.G.Obj(spell); o == nil || o.Zone != state.ZHand || o.Face() == nil {
		t.Fatal("precondition: the spell must be in seat 0's hand")
	}
	if e.G.Obj(own).Zone != state.ZBattlefield || e.G.Obj(opp).Zone != state.ZBattlefield {
		t.Fatal("precondition: one own and one opponent nonland permanent on the battlefield")
	}
	if e.G.Obj(own).Controller != 0 || e.G.Obj(opp).Controller != 1 {
		t.Fatal("precondition: the candidate controllers differ")
	}
	ppPriority(t, e)
	pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", spell)
	if pp.Reason != "" || pp.Plan == nil {
		t.Fatalf("potential cast = reason %q detail %q plan %+v, want {1}{U} priced from the own permanent target",
			pp.Reason, pp.Detail, pp.Plan)
	}
	if len(pp.Plan.Activations) != 2 {
		t.Fatalf("witness activations = %d, want the two Islands (%+v)", len(pp.Plan.Activations), pp.Plan)
	}
	// The real-pool priority walk cannot pay {4}{U} from an empty pool, so
	// this plan is the only route by which the cast is offered (the
	// lowering route the oracle uses).
	if hasCastOption(e.legalActions(0), spell) {
		t.Fatal("precondition: the cast must not be offered from the empty floating pool")
	}

	// Control: without any own nonland permanent no legal target set earns
	// the reduction, so {4}{U} is unpayable and the cast is not even a
	// potential play. This proves the assertion above depends on the own
	// target.
	e2 := handEngine(t, card(t, ttabeSrc))
	spell2 := e2.G.Zone(state.ZHand, 0)[0]
	onBoard(t, e2, 0, ttabeIsland())
	onBoard(t, e2, 0, ttabeIsland())
	onBoard(t, e2, 1, "Name:TTABE Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	ppPriority(t, e2)
	for _, pp2 := range e2.PotentialPaymentPlans(0) {
		if pp2.Action.Kind == "cast" && pp2.Action.Obj == spell2 {
			t.Fatalf("control cast without an own permanent is a potential play: reason %q detail %q plan %+v, want unpayable at {4}{U}", pp2.Reason, pp2.Detail, pp2.Plan)
		}
	}
}
