package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The pose of dirac/mana_pain_census_test.go's painOnlyScen, the plain-cast
// half of the Cabal Pit over-offer class: six untapped Cabal Pits ({T}: Add {B}. Cabal Pit deals 1 damage
// to you), {C}1 {B}1 floated, the payer at 1 life, and a plain {1}{B} spell
// (Caligo Skin-Witch's printed cost) in hand. The planner's lethal guard
// (pay/search.go) cuts every plan whose summed pain reaches the payer's life,
// so at 1 life every pit activation is refused and the floats alone must
// price the cast — which they do ({1}{B} is two units): gorge offers the
// plain cast with no activation at all.

func painPlainPose(t *testing.T, seed uint64, life int32, mc, b int32) (e *Engine, spell state.ObjID, pits []state.ObjID) {
	t.Helper()
	e, _, spell = newFixtureDeck(t, seed, "Name:One Probe\nManaCost:1 B\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n")
	for i := 0; i < 6; i++ {
		id := onBoardCard(t, e, 0, corpusCardOrFatal(t, "Cabal Pit"))
		e.G.Obj(id).SummonSick = false
		if o := e.G.Obj(id); o.Tapped || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: pit %d tapped=%v zone=%v", id, o.Tapped, o.Zone)
		}
		pits = append(pits, id)
	}
	e.G.Players[0].Life = life
	e.G.Players[0].Pool[state.MC] = mc
	e.G.Players[0].Pool[state.ManaIndex('B')] = b
	if e.G.Players[0].Life != life || e.G.Players[0].Pool[state.MC] != mc || e.G.Players[0].Pool[state.ManaIndex('B')] != b {
		t.Fatal("precondition: the pool or the life did not take")
	}
	if o := e.G.Obj(spell); o.Zone != state.ZHand {
		t.Fatalf("precondition: spell zone = %v, want the hand", o.Zone)
	}
	return e, spell, pits
}

// TestPaymentPlanPainPlainCastFloatsPay: at the pose the floats alone pay
// {1}{B}, so the planner prices the cast with no activation and the priority
// decision offers it — dirac offers the same cast (its census holds the floats
// alone too), and the ticket's "plain cast is the remaining offer" was this
// agreement, not a divergence.
func TestPaymentPlanPainPlainCastFloatsPay(t *testing.T) {
	t.Parallel()
	e, spell, _ := painPlainPose(t, 9843, 1, 1, 1)
	out := e.PlanCastPayment(0, paymentCast(spell))
	if out.Plan == nil {
		t.Fatalf("planner refuses the plain cast the floats alone pay (pool %+v, life %d): %+v",
			e.G.Players[0].Pool, e.G.Players[0].Life, out)
	}
	if len(out.Plan.Activations) != 0 {
		t.Fatalf("plan taps %d sources, want the floats alone: %+v", len(out.Plan.Activations), out.Plan.Activations)
	}
	d := ppPriority(t, e)
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			return
		}
	}
	t.Fatalf("priority does not offer the plain cast: %+v", d.Options)
}

// TestPaymentPlanPainPlainCastShortFloats: the same board at 1 life with the
// {B} float missing ({C}1 floated, or an empty pool): every {B} unit needs a
// pit activation whose pain reaches the life, so the planner refuses the cast
// (insufficient) and the priority decision does not offer it — the census's
// lethal guard holds on the plain-cast route too.
func TestPaymentPlanPainPlainCastShortFloats(t *testing.T) {
	t.Parallel()
	for _, mc := range []int32{1, 0} {
		e, spell, _ := painPlainPose(t, 9844, 1, mc, 0)
		out := e.PlanCastPayment(0, paymentCast(spell))
		if out.Plan != nil {
			t.Fatalf("planner prices the cast at 1 life with pool %+v: %+v", e.G.Players[0].Pool, out.Plan)
		}
		if out.Reason != "insufficient" {
			t.Fatalf("reason = %q, want insufficient (pool %+v)", out.Reason, e.G.Players[0].Pool)
		}
		d := ppPriority(t, e)
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == spell {
				t.Fatalf("priority offers the plain cast at 1 life with pool %+v: %+v", e.G.Players[0].Pool, d.Options)
			}
		}
	}
}

// TestPaymentPlanPainPlainCastSummedGuard: the summed half of the guard. At 2
// life one pit activation (pain 1 < 2) is a plan unit and prices the cast with
// the {C} float; two activations would sum to the life and be cut — the plan
// taps exactly one pit. (dirac's payment census prices the same at
// painTight's budget: TestPainOnlyCensusPlainCastFloats.)
func TestPaymentPlanPainPlainCastSummedGuard(t *testing.T) {
	t.Parallel()
	e, spell, pits := painPlainPose(t, 9845, 2, 1, 0)
	out := e.PlanCastPayment(0, paymentCast(spell))
	if out.Plan == nil {
		t.Fatalf("planner refuses the cast one activation pays (pool %+v, life %d): %+v",
			e.G.Players[0].Pool, e.G.Players[0].Life, out)
	}
	if len(out.Plan.Activations) != 1 {
		t.Fatalf("plan taps %d sources, want one pit activation: %+v", len(out.Plan.Activations), out.Plan.Activations)
	}
	src := out.Plan.Activations[0].Source
	if src != pits[0] && src != pits[1] && src != pits[2] && src != pits[3] && src != pits[4] && src != pits[5] {
		t.Fatalf("activation source %d is no Cabal Pit: %+v", src, out.Plan.Activations)
	}
}
