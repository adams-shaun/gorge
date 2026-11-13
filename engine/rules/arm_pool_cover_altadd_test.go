package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// armAltAddSpell costs {B} and carries a mandatory either-or additional cost:
// sacrifice an Elf, or pay {1}. It targets an artifact.
const armAltAddSpell = "Name:Arm Alt Add Spell\nManaCost:B\nTypes:Sorcery\n" +
	"K:AlternateAdditionalCost:Sac<1/Elf/an Elf>:1\n" +
	"A:SP$ Destroy | SpellDescription$ Destroy target artifact. | ValidTgts$ Artifact\nOracle:x\n"

// The consumption-only arm's pool-cover pre-gate (planPoolCoversPrintedCost)
// only composes the printed mana cost. An either-or additional cost
// (AlternateAdditionalCost) is folded in later, by the cast's own
// announcement, and potentialModeCastPlan completes the retained mana plan
// with it (potentialAltAddSacrificePlan, potentialAltAddPlan). A part can
// therefore consume a permanent the pool-covered mana plan leaves alone --
// here the only target is the untapped Treasure whose own sacrifice pays the
// {1} part -- so the pre-gate must not answer for such a cast: the either-or
// arms must stay reachable through the planner.
func TestArmPoolCoverKeepsAlternateAdditionalCostArms(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9417, ppFiller)
	spell := handCard(e, card(t, armAltAddSpell), 0)
	gem := onBoard(t, e, 0, tpTreasure)
	e.G.Step = state.StepMain1
	ppPriority(t, e)
	// Preconditions: the keyword parsed into its two parts, the cast names
	// the arm's shape, and the floating pool pays the printed {B} alone.
	parts := altAddCostParts(e.G.Obj(spell).Face())
	if len(parts) != 2 {
		t.Fatalf("alternate additional cost parsed %v, want the Elf sacrifice and {1}", parts)
	}
	if detail := e.paymentPlanCastShapeDetail(0, spell); detail != "shape:optional_cost" {
		t.Fatalf("shape detail %q, want shape:optional_cost", detail)
	}
	opt := decision.Option{Kind: "cast", Obj: spell}
	e.G.Players[0].Pool[state.MB] = 1
	if !e.planPoolCoversPrintedCost(0, opt) {
		t.Fatal("the floating {B} does not cover the printed {B}")
	}
	// The pool cannot pay the {1} part, and no Elf is on the board: only
	// sacrificing the target for its own mana completes the cast, so the arm
	// must plan rather than let the pre-gate answer.
	if !e.targetPaidNeedsPlannedPayment(0, spell) {
		t.Fatal("the pool-cover pre-gate declined the cast whose either-or part consumes its target")
	}
	e.paidManaCensus = true
	e.EnsurePaymentActions()
	d := e.Pending()
	if !ppHasAction(d, spell) {
		t.Fatal("no planned payment for the cast whose either-or part sacrifices its target")
	}
	submitPaymentPlan(t, e, d, ppAction(t, d, spell))
	for guard := 0; guard < 20; guard++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		if d.Kind == decision.KChoose {
			for _, o := range d.Options {
				in.Choices = append(in.Choices, o.Index)
				if len(in.Choices) >= max(d.Min, 1) {
					break
				}
			}
		} else {
			in.Choices = []int{d.Options[0].Index}
		}
		if err := e.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.G.Stack) != 1 || e.G.Obj(gem).Zone == state.ZBattlefield {
		t.Fatalf("stack %v, gem in zone %v: want the spell cast and the gem sacrificed to pay",
			e.G.Stack, e.G.Obj(gem).Zone)
	}
}
