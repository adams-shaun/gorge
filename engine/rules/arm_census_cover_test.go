package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// armCoverSpell is the target-dependent-cost cast of the consumption-only
// arm's fixture (Luminous Rebuke's shape): printed {4}{W}, {3} less for a
// tapped-creature target.
const armCoverSpell = "Name:Arm Census Cover Spell\nManaCost:4 W\nTypes:Instant\n" +
	"S:Mode$ ReduceCost | Type$ Spell | ValidCard$ Card.Self | Amount$ 3 | EffectZone$ All | ValidTarget$ Creature.tapped | Description$ This spell costs {3} less to cast if it targets a tapped creature.\n" +
	"A:SP$ Destroy | SpellDescription$ Destroy target creature. | ValidTgts$ Creature\nOracle:x\n"

// The consumption-only planner arm answers a cost the option set provably
// cannot fund without the search (planCoverInsufficient): the floating pool
// plus, per census source, its largest alternative is a superset of what
// any plan may spend, so pips beyond it mean the planner's insufficient.
// The assertions pin the bound's own answers (which colours and amounts it
// reads, and that it declines every shape the planner itself declines);
// the arm's verdicts flowing through it are the consumption-only test's
// contract.
func TestArmCensusCoverGate(t *testing.T) {
	t.Parallel()
	plain := func(w, u, b, r, g, c, gen int32) cost.Cost {
		return cost.Cost{Colored: state.Mana{w, u, b, r, g, c}, Generic: gen}
	}
	e, _, _ := newFixtureDeck(t, 9415, ppFiller)
	fourW := plain(0, 0, 0, 0, 0, 0, 4)
	fourW.Colored[state.MW] = 1
	oneG := plain(0, 0, 0, 0, 0, 0, 1)
	oneG.Colored[state.MG] = 1
	onBoard(t, e, 0, "Name:Arm Census Cover Forest\nTypes:Basic Land Forest\nOracle:x\n")
	onBoard(t, e, 0, "Name:Arm Census Cover Forest\nTypes:Basic Land Forest\nOracle:x\n")
	e.planConsumptionOnly = true
	if !e.planCoverInsufficient(0, fourW) {
		t.Fatal("no white source and two green ones cannot fund {4}{W}")
	}
	if e.planCoverInsufficient(0, oneG) {
		t.Fatal("two forests fund {1}{G}")
	}
	e.G.Players[0].Pool[state.MW] = 1
	tightW := plain(0, 0, 0, 0, 0, 0, 1)
	tightW.Colored[state.MW] = 1
	if e.planCoverInsufficient(0, tightW) {
		t.Fatal("floating white funds {1}{W} over the two forests")
	}
	overW := plain(0, 0, 0, 0, 0, 0, 3)
	overW.Colored[state.MW] = 1
	if !e.planCoverInsufficient(0, overW) {
		t.Fatal("one white pip and three generic beyond the pool's and forests' three mana is unfundable")
	}
	e.G.Players[0].Pool[state.MW] = 0
	onBoard(t, e, 0, "Name:Arm Census Cover Plains\nTypes:Basic Land Plains\nOracle:x\n")
	for i := 0; i < 2; i++ {
		onBoard(t, e, 0, "Name:Arm Census Cover Forest\nTypes:Basic Land Forest\nOracle:x\n")
	}
	if e.planCoverInsufficient(0, fourW) {
		t.Fatal("four forests and a plains fund {4}{W}")
	}
	// Every shape the planner itself declines is not read: the gate leaves
	// it to the planner's own decline.
	if e.planCoverInsufficient(0, cost.Cost{X: 1}) {
		t.Fatal("an unfolded X is not a plain cost")
	}
	if e.planCoverInsufficient(0, cost.Cost{Hybrid: []cost.ManaPair{{A: 'R', B: 'G'}}}) {
		t.Fatal("a hybrid pip is not a plain cost")
	}
	if e.planCoverInsufficient(0, cost.Cost{Snow: 1}) {
		t.Fatal("snow mana is not a plain cost")
	}
	if e.planCoverInsufficient(0, cost.Cost{Sac: []cost.CostPart{{N: 1, Spec: "Creature"}}}) {
		t.Fatal("a sacrifice part is not a plain cost")
	}

	// Through the arm: the same cast's printed cost is unfundable over two
	// forests (the gate answers it without the search) and fundable over
	// the grown board; the arm's verdict is unchanged either way -- no
	// planned payment, the shape census still names the cast.
	for _, tc := range []struct {
		name     string
		lands    int
		plains   int
		unfunded bool
	}{
		{"unfundable_printed_cost", 2, 0, true},
		{"funded_printed_cost", 4, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			te, _, _ := newFixtureDeck(t, 9416, ppFiller)
			spell := handCard(te, card(t, armCoverSpell), 0)
			for i := 0; i < tc.lands; i++ {
				onBoard(t, te, 0, "Name:Arm Census Cover Forest\nTypes:Basic Land Forest\nOracle:x\n")
			}
			for i := 0; i < tc.plains; i++ {
				onBoard(t, te, 0, "Name:Arm Census Cover Plains\nTypes:Basic Land Plains\nOracle:x\n")
			}
			te.G.Step = state.StepMain1
			ppPriority(t, te)
			printed := cost.Cost{Generic: 4}
			printed.Colored[state.MW] = 1
			if got := te.planCoverInsufficient(0, printed); got != tc.unfunded {
				t.Fatalf("printed-cost cover gate = %v, want unfunded %v", got, tc.unfunded)
			}
			if detail := te.paymentPlanCastShapeDetail(0, spell); detail != "shape:target_dependent_cost" {
				t.Fatalf("shape detail %q, want shape:target_dependent_cost", detail)
			}
			te.EnsurePaymentActions()
			d := te.Pending()
			if ppHasAction(d, spell) {
				t.Fatal("a planned payment was offered")
			}
		})
	}
}
