package rules

import "testing"

// TestCensusFundsAnUnrelaxableGap: Starfleet Academy's "{5}, {T}: Add X mana
// of any one color" has no relaxed alternative (X is unbounded), but its fee
// is still mana the prefix search must fund by tapping other sources first
// (CR 602.2b). The census must report that fee even though the relaxed proof
// declines the ability, or the search starts each gap activation with an
// empty pool and can never pay it.
func TestCensusFundsAnUnrelaxableGap(t *testing.T) {
	t.Parallel()
	e := tapXBoard(t, []string{"Starfleet Academy", "Plains", "Plains", "Plains", "Plains", "Plains"})
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || len(census.gaps) == 0 {
		t.Fatalf("census complete=%v gaps=%v, want Starfleet Academy's unbounded mana ability to be a gap", census.complete, census.gaps)
	}
	if got := census.fees(); got < 5 {
		t.Fatalf("census fees = %d, want at least Starfleet Academy's {5} activation fee", got)
	}
}
