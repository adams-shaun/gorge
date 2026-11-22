package oracle

import "testing"

// CR 605.3a with CR 601.2g: Energy Refractor's "{2}: Add one mana of any color"
// is a mana ability payable from other sources, so Plains and Racers' Ring fund
// the blue that, with the two Islands, pays Blue Sun's Zenith ({X}{U}{U}{U}) at
// X=0. The cast must be offered (decision 375 of pair 122, tail-02).
func TestMBCBlueSunSZenithReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fee_only_mana_funds_blue_sun_zenith_cast")
}
