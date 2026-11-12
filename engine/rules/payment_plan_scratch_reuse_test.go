package rules

// The payment planner's per-call table scratch (pay.PlanScratch) is recycled
// across plan calls on one engine: the withheld census rows, the phase
// tables and the class table a plan call builds are rebuilt over the same
// storage every time. A reused scratch must serve the same plan for the same
// query a fresh build did, so this test plans cast A, cast B (a different
// cost, whose search rewrites the scratch with different tables) and cast A
// again on the SAME engine and requires the two A outcomes to be the same
// plan, bit for bit. Fresh allocation is equally correct, so this is a guard
// against aliasing and clobbering, not a defect repro: it also checks the
// escape contract by backing identity (an adopted plan's activation list is
// never the shared candidate-activation storage), and its companion
// evidence is the before/after profile pair and the enginebench verify smoke.

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
		// The escape contract says the returned plan's activation list is
		// its own storage: snapshot it and require it to survive the
		// intervening cast-B search, whose candidates overwrite the shared
		// candidate-activation backing (witnessInto's sc.acts) in place
		// whenever they fit it. A plan that still pointed into the scratch
		// would read cast B's witness here (vacuous only in the shape where
		// B's search grows the backing past A's, leaving A's copy
		// abandoned-but-intact; the backing check below is what has no
		// such escape).
		a1Snap := make([]decision.PaymentActivation, len(a1.Plan.Activations))
		copy(a1Snap, a1.Plan.Activations)
		b := pay.PlanPaymentCost(asPayer(e), 0, paymentCast(spell), costB, nil)
		if !reflect.DeepEqual(a1Snap, a1.Plan.Activations) {
			t.Fatalf("the first A plan's activations changed while cast B planned over the same scratch; the plan still points into the reused search scratch\nwas:  %s\nnow:  %s",
				lrPlanString(a1Snap), lrPlanString(a1.Plan.Activations))
		}
		a2 := pay.PlanPaymentCost(asPayer(e), 0, paymentCast(spell), costA, nil)
		a3 := pay.PlanPaymentCost(asPayer(e), 0, paymentCast(spell), costA, nil)
		for _, got := range []pay.PlanOutcome{a1, b, a2, a3} {
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
		if !pay.PlanOutcomeEqual(a2, a3) || !reflect.DeepEqual(*a2.Plan, *a3.Plan) {
			t.Fatalf("two back-to-back identical queries disagreed over the reused scratch\nfirst:  %s\nsecond: %s", lrPlanString(a2.Plan.Activations), lrPlanString(a3.Plan.Activations))
		}
		// The escape contract itself, by backing identity: an adopted
		// plan's activation list is freshly allocated while the previous
		// call's adopted plan is still alive, so the two backings cannot
		// be one allocation. If the search stored its candidate list
		// without the adopt copy, both plans would point into the same
		// candidate-activation storage — witnessInto hands every (grown)
		// backing back to the scratch, and this query pair's searches
		// share it (a call's growth becomes the next call's starting
		// capacity, and the second call's best fits the capacity the
		// identical first call left, so no growth separates them).
		back2 := reflect.ValueOf(a2.Plan.Activations).Pointer()
		back3 := reflect.ValueOf(a3.Plan.Activations).Pointer()
		if back2 == 0 || back3 == 0 {
			t.Fatal("empty activation backing, though every plan here has steps")
		}
		if back2 == back3 {
			t.Fatalf("two live plans share an activation backing; an adopted plan still points into the reused search scratch (the escape contract pay.SearchScratch's doc states)")
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
