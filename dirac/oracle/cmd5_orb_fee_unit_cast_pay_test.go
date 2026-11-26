package oracle

import "testing"

// TestCmd5OrbFeeUnitCastPayPlan: the seat casts Shivan Devastator ({X}{R}) and
// pays it silently. Gorge's composed-cost plan prices the census alone
// (paid_mana_census.go: a generic or coloured mana fee is the cast window's
// own price), so the kernel's cast-payment plan prices no Orb of Dragonkind
// fee unit beside the census's. It used to activate a Treasure to fill the
// pool, draw the Orb's {1} fee from it (feeOK, CR 605.3b) and float the
// restricted unit, tapping the Orb and keeping a Treasure gorge's plan
// sacrificed. The cast-payment pass now holds the fee unit out the X-witness's
// way (availNoFeeUnit, mana_window.go); only the window's funded fee-gap arms
// admit it.
func TestCmd5OrbFeeUnitCastPayPlan(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_orb_fee_unit_cast_pay_plan", 1223, "Cards[307].Pos")
	requireCmd5ReproClean(t, res, 1223)
}
