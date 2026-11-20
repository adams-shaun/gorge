package dirac

import "testing"

// TestFeeOrderFundsBeforeTheComboFilter: Super Speed's {R} beside Baxter
// Building's {4},{T}: four mana in any combination, funded by three Forests and
// an Avengers Tower whose second, Hero-only ability adds a branch to its tap.
// The plain order taps the filter's free {C} first and spends the node budget
// on the funders' orderings under it; gorge's phased fee order floats the
// funding first, so the cast is offered (CR 601.2g, 605.3b).
func TestFeeOrderFundsBeforeTheComboFilter(t *testing.T) {
	for _, forests := range []int{3, 4} {
		sc, id := synthScen(t, "std-msh", "Super Speed", "Forest", "Baxter Building", "Guerrilla Gorilla", "Avengers Tower")
		sc.perm(id["Avengers Tower"], 0)
		for i := 0; i < forests; i++ {
			sc.perm(id["Forest"], 0)
		}
		sc.perm(id["Baxter Building"], 0)
		sc.perm(id["Guerrilla Gorilla"], 0)
		c := sc.hand(id["Super Speed"], 0)
		if acts := sc.castOptions(c); len(acts) == 0 {
			t.Fatalf("%d Forests and an Avengers Tower: Super Speed is not offered through Baxter Building's filter", forests)
		}
	}
}
