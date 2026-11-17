package rules

// A searched payment alternative (EnsurePaymentPlanAlternatives) must obey the
// same rule plan 0 does under a may-play any-color rider: a folded witness that
// prices the relaxed coloured pip generic and funds it from a colourless-only
// ability validates (the settle reads the folded cost) but cannot pay the real
// pip -- "any color" never includes colourless (CR 107.4c) -- so the cast would
// revert with the card still in the graveyard and the pool holding the
// stranded {C}. Only alternatives whose production carries a coloured unit may
// be offered.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// planProducesAColouredUnit reports whether any activation of plan produces a
// coloured unit (any of WUBRG; CR 107.4c leaves colourless out).
func planProducesAColouredUnit(plan decision.PaymentPlan) bool {
	for _, a := range plan.Activations {
		for c := state.MW; c <= state.MG; c++ {
			if a.Produces[c] > 0 {
				return true
			}
		}
	}
	return false
}

// TestMayPlayAnyColorSearchedAlternativeFundsTheColouredPipToo: beside a
// Mountain, a printed-red land and one dual-ability land, the folded {R}
// may-play cast's plan 0 activates the Mountain, and the searched alternative
// that activates the dual land's colourless ability must not be offered -- its
// production cannot pay the printed {R} under the rider. Every alternative
// that IS offered must carry a coloured unit, validate, and execute: Submit
// stacks the creature, taps the source and empties the pool.
func TestMayPlayAnyColorSearchedAlternativeFundsTheColouredPipToo(t *testing.T) {
	t.Parallel()
	e, creature := mayPlayRiderBoard(t, "MayPlayIgnoreColor$ True | ", "R", 1)
	ridge := onBoard(t, e, 0, "Name:Test Ridge\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Red.\nOracle:x\n")
	coast := onBoard(t, e, 0, "Name:Test Coast\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Colourless.\n"+
		"A:AB$ Mana | Cost$ T | Produced$ Combo G U | SpellDescription$ Green or blue.\nOracle:x\n")
	e.pending = nil
	e.askPriority(0)
	if o := e.G.Obj(creature); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("creature in zone %v, want the graveyard", o.Zone)
	}
	if e.G.Players[0].Pool != (state.Mana{}) {
		t.Fatalf("pool = %v, want empty", e.G.Players[0].Pool)
	}
	if o := e.G.Obj(coast); o == nil || o.Tapped {
		t.Fatalf("the dual-ability land is not an untapped source: %+v", o)
	}
	acts := e.EnsurePaymentPlanAlternatives()
	var a decision.PaymentAction
	found := false
	for _, x := range acts {
		if x.Cast.Object == creature {
			a, found = x, true
		}
	}
	if !found {
		t.Fatalf("no payment action for %d under the any-color rider", creature)
	}
	if len(a.Plans) == 0 {
		t.Fatalf("payment action for %d offers no plan", creature)
	}
	for i, p := range a.Plans {
		if err := e.ValidateCastPayment(0, a.Cast, p); err != nil {
			t.Fatalf("ValidateCastPayment(plan %d): %v", i, err)
		}
		if !planProducesAColouredUnit(p) {
			t.Fatalf("plan %d funds the pip without a coloured unit (colourless-only production): %+v", i, p)
		}
	}
	if len(a.Plans) < 2 {
		return
	}
	// An alternative that is offered must also execute: Submit the last one.
	d := e.Pending()
	last := a.Plans[len(a.Plans)-1]
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Payment: &decision.PaymentSelection{ActionID: a.ID, Plan: last}}); err != nil {
		t.Fatalf("Submit alternative plan: %v", err)
	}
	if got := e.G.Obj(creature).Zone; got != state.ZStack {
		t.Fatalf("creature in zone %v after the alternative payment, want the stack", got)
	}
	if o := e.G.Obj(ridge); o == nil || !o.Tapped {
		t.Fatalf("the alternative's source is not tapped: %+v", o)
	}
	if pool := e.G.Players[0].Pool; pool != (state.Mana{}) {
		t.Fatalf("pool = %v after the alternative payment, want empty", pool)
	}
}
