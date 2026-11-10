package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGatePairPotentialSums: with the gate on, gorge lists both of Whisperer
// of the Wilds' mana abilities and its potential walk (rules potentialManaFor)
// sums them, so Whisperer alone is 3 {G} of potential; the planner's per-tap
// pick (availFor) is still the larger one, 2 {G}. With the gate off both are
// the census {G}.
func TestGatePairPotentialSums(t *testing.T) {
	set := cardsettest.Set(t, "cmd5")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	for _, n := range []string{"Whisperer of the Wilds", "Leatherback Baloth", "Forest"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s not in the cmd5 table", n)
		}
	}
	measure := func(gate bool) (pot, plan avail) {
		sc := newScenTableLib(t, tbl, id["Forest"], Config{})
		if !sc.k.gen.anyGatePair {
			t.Fatal("anyGatePair unset: Whisperer is not a gated pair in the cmd5 prep")
		}
		sc.perm(id["Whisperer of the Wilds"], 0)
		if gate {
			b := sc.perm(id["Leatherback Baloth"], 0)
			sc.s.perms[sc.slot(b)].flags |= pfTapped // the gate, not a mana source
		}
		other := payDesc{class: pdOther}
		sc.s.availPotentialFor(sc.k, 0, &pot, &other)
		sc.s.availFor(sc.k, 0, &plan)
		return
	}
	pot, plan := measure(true)
	if pot.total != 3 {
		t.Errorf("gate on: potential %d units, want 3 ({G} plus {G}{G}, each listed ability on its own)", pot.total)
	}
	g3 := cost(0, kop.G, kop.G, kop.G)
	if !pot.affords(&g3, pipMaskOf(&g3), 0) {
		t.Error("gate on: the potential bound does not reach {G}{G}{G}")
	}
	if plan.total != 2 {
		t.Errorf("gate on: planner avail %d units, want 2 (the larger alternative of one tap)", plan.total)
	}
	pot, plan = measure(false)
	if pot.total != 1 || plan.total != 1 {
		t.Errorf("gate off: potential %d, planner %d, want 1 and 1", pot.total, plan.total)
	}
}
