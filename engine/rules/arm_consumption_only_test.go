package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// acoSpell is a target-dependent-cost spell (Luminous Rebuke's shape): the
// printed cost, less three for a tapped-creature target.
const acoSpell = "Name:Arm Consumption Spell\nManaCost:4 W\nTypes:Instant\n" +
	"S:Mode$ ReduceCost | Type$ Spell | ValidCard$ Card.Self | Amount$ 3 | EffectZone$ All | ValidTarget$ Creature.tapped | Description$ This spell costs {3} less to cast if it targets a tapped creature.\n" +
	"A:SP$ Destroy | SpellDescription$ Destroy target creature. | ValidTgts$ Creature\nOracle:x\n"

// acoElf is a mana source that sacrifices itself for one mana of any colour:
// the cast's only legal target and, without another white source, the only
// way to pay it.
const acoElf = "Name:Arm Consumption Elf\nTypes:Creature Elf\nPower:1\nToughness:1\n" +
	"A:AB$ Mana | Cost$ Sac<1/CARDNAME> | Produced$ Any | Amount$ 1 | SpellDescription$ Sacrifice this creature: Add one mana of any colour.\nOracle:x\n"

const acoForest = "Name:Arm Consumption Forest\nTypes:Basic Land Forest\nOracle:x\n"
const acoPlains = "Name:Arm Consumption Plains\nTypes:Basic Land Plains\nOracle:x\n"

// The shape-cast planner arm reads its plan only for whether it consumes
// something, so a plan that consumes nothing on a cast with no either-or
// additional cost parts is declined by the arm itself and its witness needs
// no offer-gate walk (planWitnessOffersNeeded) -- while the keep-sources
// plan, a consuming plan, and an either-or part still pay for the gate.
// The verdicts below are the contract the waiver must not move: with a
// plain white source the Rebuke-shaped cast is payable without consuming
// its target and carries no planned payment; without one its only payable
// plan sacrifices the target itself and the planned payment carries it.
func TestArmConsumptionOnlyWaivesOfferGate(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9413, ppFiller)
	free := decision.PaymentPlan{}
	consuming := decision.PaymentPlan{Activations: []decision.PaymentActivation{{Consequence: &decision.PaymentConsequence{Sacrifice: true}}}}
	partsFace := card(t, "Name:Arm Consumption Either\nManaCost:W\nTypes:Artifact\n"+
		"K:AlternateAdditionalCost:Sac<1/Artifact/an artifact>:2\nOracle:x\n")
	spellFace := card(t, acoSpell).Faces[0]
	e.planConsumptionOnly = true
	if !e.planWitnessOffersNeeded(partsFace.Faces[0], free) {
		t.Fatal("an either-or additional cost's witness fallback needs the offer gate")
	}
	e.planKeepSources = true
	if !e.planWitnessOffersNeeded(spellFace, free) {
		t.Fatal("the keep-sources plan needs the offer gate")
	}
	e.planKeepSources = false
	if !e.planWitnessOffersNeeded(spellFace, consuming) {
		t.Fatal("a consuming plan needs the offer gate")
	}
	if e.planWitnessOffersNeeded(spellFace, free) {
		t.Fatal("a consumption-only query's nothing-consuming plan needs no offer gate")
	}
	e.planConsumptionOnly = false
	if !e.planWitnessOffersNeeded(spellFace, free) {
		t.Fatal("the waiver must not apply outside a consumption-only query")
	}

	for _, tc := range []struct {
		name       string
		withPlains bool
		planned    bool
	}{
		{"plain_white_source", true, false},
		{"target_is_the_only_white_source", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			te, _, _ := newFixtureDeck(t, 9414, ppFiller)
			spell := handCard(te, card(t, acoSpell), 0)
			elf := onBoard(t, te, 0, acoElf)
			for i := 0; i < 4; i++ {
				onBoard(t, te, 0, acoForest)
			}
			if tc.withPlains {
				onBoard(t, te, 0, acoPlains)
			} else {
				onBoard(t, te, 0, acoForest)
			}
			te.G.Obj(elf).Tapped = true
			te.G.Step = state.StepMain1
			ppPriority(t, te)
			if detail := te.paymentPlanCastShapeDetail(0, spell); detail != "shape:target_dependent_cost" {
				t.Fatalf("shape detail %q, want shape:target_dependent_cost", detail)
			}
			te.EnsurePaymentActions()
			d := te.Pending()
			if !tc.planned {
				if ppHasAction(d, spell) {
					t.Fatal("a planned payment was offered for a cast payable without consuming its target")
				}
				return
			}
			if !ppHasAction(d, spell) {
				t.Fatal("no planned payment for the cast whose only white mana is its own target")
			}
			submitPaymentPlan(t, te, d, ppAction(t, d, spell))
			for guard := 0; guard < 20; guard++ {
				d := te.Pending()
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
				if err := te.Submit(in); err != nil {
					t.Fatal(err)
				}
			}
			if te.G.Obj(elf).Zone == state.ZBattlefield {
				t.Fatal("elf still on the battlefield: the payment did not sacrifice the target")
			}
			if len(te.G.Stack) != 1 {
				t.Fatalf("stack %v, want the cast", te.G.Stack)
			}
		})
	}
}
