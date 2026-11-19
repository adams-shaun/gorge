package oracle

import "testing"

// TestMBCBloomTenderReprosLockstep is the each-colour census gap's shrink: a
// trigger's own optional mana cost beside an each-colour mana source
// (Bloom Tender, "Add one mana of each colour among permanents you control").
// gorge's payment-bound potential walk prices an open production unbounded in
// every colour (rules potential.go producedOpen over addPotentialMana), while
// the census prices the colours the board has now, so beside it the ask offers
// pay and decline. The recording is synth std-ecl decision 360: gorge offered
// trigger_cost_pay and the kernel posed decline only (options options). The
// shape's fold lives in dirac/mana_dyntrigcost.go (potentialDynOpen's
// each-colour arm and the strict sources flag, anyLiveSource).
func TestMBCBloomTenderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "each_color_among_valid_pay_unaffordable")
}
