package rules

// The payment planner's per-call table scratch (pay.PlanScratch) is recycled
// across plan calls on one engine: the withheld census rows, the phase
// tables and the class table a plan call builds are rebuilt over the same
// storage every time. A reused scratch must serve the same plan for the same
// query a fresh build did, so this test plans cast A, cast B (a different
// cost, whose search rewrites the scratch with different tables) and cast A
// again on the SAME engine and requires the two A outcomes to be the same
// plan, bit for bit. Fresh allocation is equally correct, so this is a guard
// against aliasing and clobbering, not a defect repro: its companion
// evidence is the before/after profile pair and the enginebench verify smoke
// (see the brief).

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestPaymentPlanScratchReuseStable(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T) {
		e, _, spell := newFixtureDeck(t, 9331, srchSpell("U"))
		p1 := onBoard(t, e, 0, lrPlains)
		p2 := onBoard(t, e, 0, lrPlains)
		tr := lrTreasure(t, e)
		vault := onBoardCard(t, e, 0, corpusCard(t, "Mana Vault"))
		// Precondition: every source the plans may name is really on the
		// battlefield (and not summoning sick, so the census admits it).
		for _, id := range []state.ObjID{p1, p2, tr, vault} {
			if e.G.Obj(id) == nil {
				t.Fatalf("source %d missing from the battlefield", id)
			}
			e.G.Obj(id).SummonSick = false
		}
		costA := Cost{Generic: 2}
		costA.Colored[state.MW] = 2
		costB := Cost{Generic: 2}
		costB.Colored[state.MW] = 1
		// Phase 1 (the two Plains alone, 2 mana) cannot pay either cost, so
		// every plan here is a phase-2 plan over the reused scratch's
		// last-resort tables; a disclosed consequence is its marker.
		a1 := pay.PlanPaymentCost(asPayer(e), 0, paymentCast(spell), costA, nil)
		b := pay.PlanPaymentCost(asPayer(e), 0, paymentCast(spell), costB, nil)
		a2 := pay.PlanPaymentCost(asPayer(e), 0, paymentCast(spell), costA, nil)
		for _, got := range []pay.PlanOutcome{a1, b, a2} {
			if got.Plan == nil || got.Reason != "" {
				t.Fatalf("no plan (reason %q detail %q), want a phase-2 plan", got.Reason, got.Detail)
			}
			if len(got.Plan.Activations) == 0 {
				t.Fatalf("empty witness for reason %q", got.Reason)
			}
			lastResort := false
			for _, a := range got.Plan.Activations {
				lastResort = lastResort || a.Consequence != nil
			}
			if !lastResort {
				t.Fatalf("plan %s names no last-resort source, though phase 1 (two Plains) cannot pay", lrPlanString(got.Plan.Activations))
			}
		}
		if !pay.PlanOutcomeEqual(a1, a2) {
			t.Fatalf("the reused scratch served different outcomes for the same query\nfirst:  %s\nsecond: %s", lrPlanString(a1.Plan.Activations), lrPlanString(a2.Plan.Activations))
		}
		if !reflect.DeepEqual(*a1.Plan, *a2.Plan) {
			t.Fatalf("the reused scratch served different plans for the same query\nfirst:  %s\nsecond: %s", lrPlanString(a1.Plan.Activations), lrPlanString(a2.Plan.Activations))
		}
		// Precondition for the guard's value: the middle search really
		// rewrote the scratch with different tables (otherwise the two A
		// calls would agree even through a clobbered scratch), and the two
		// casts' witness steps differ. The step-list comparison is what
		// catches the aliasing shape: a plan whose activation list still
		// points into the reused scratch reads whatever the LAST search
		// wrote there, so A's steps and B's steps would come out equal.
		if reflect.DeepEqual(a1.Plan.Activations, b.Plan.Activations) {
			t.Fatal("cast A and cast B planned the same witness steps; the test would not detect a clobbered scratch")
		}
		// The escape contract (the returned search's best plan never points
		// into its scratch) is what makes the two A outcomes trustworthy:
		// each call's disclosed consequences are its own fresh allocations,
		// and two live plans never share one. Equality here means one of
		// them still reads the reused scratch, whose slots the last search
		// rewrote.
		for i := range a1.Plan.Activations {
			x, y := a1.Plan.Activations[i].Consequence, a2.Plan.Activations[i].Consequence
			var z *decision.PaymentConsequence
			if i < len(b.Plan.Activations) {
				z = b.Plan.Activations[i].Consequence
			}
			if (x != nil && (x == y || x == z)) || (y != nil && y == z) {
				t.Fatalf("live plans share a consequence pointer at step %d; one of them reads the reused scratch", i)
			}
		}
		t.Logf("cast A (first == second) and cast B agree through a reused scratch: A %s, B %s",
			lrPlanString(a1.Plan.Activations), lrPlanString(b.Plan.Activations))
	}
	t.Run("plain", run)
	// The verify mode's count-rank cross-check recomputes every complete
	// candidate's witness and rank over the reused scratch: a stale class
	// span or an aliased candidate activation shows up as a panic here.
	prev := walkCacheVerify
	walkCacheVerify = true
	defer func() { walkCacheVerify = prev }()
	t.Run("verify", run)
}
