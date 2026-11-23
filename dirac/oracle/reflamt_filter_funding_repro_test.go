package oracle

import "testing"

// A live Incubation Druid (reflected Amount$) leaves gorge's census
// unrelaxable even beside a plain fee filter (Boros Signet): the Druid has no
// relaxed alternative, so the unrelaxable census clears its paid list and the
// Signet's {1} fee is no funding (potential_plan.go paymentPlanCensus.fees).
// gorge's prefix search then activates the Druid, finds the Signet's fee
// unfunded by any covered tap, and prices neither the Druid's own {3}{G}{G}
// Adapt (tail-55 pair 141, seed 643597) nor Gnottvold Slumbermound's
// {3}{R}{G}{G} sacrifice ability (pair 71). The kernel's filter search funded
// the Signet's fee as on a relaxable census and offered both.
func TestReflAmtFilterFeeUnfundedBesideUnrelaxableCensusLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_filter_fee_unfunded_beside_unrelaxable_census")
}

func TestReflAmtFilterFeeUnfundedBesideUnrelaxableCensusSacrificeLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_filter_fee_unfunded_beside_unrelaxable_census_sacrifice")
}
