package oracle

import "testing"

// A free cast (Urza, Lord High Artificer's "{5}: ... you may play that card
// without paying its mana cost") still pays a target-dependent raise:
// an opponent's Kasmina, Enigmatic Mentor taxes the empty cost {2} the moment
// the spell targets a creature she controls (CR 601.2f reprices on the CR
// 601.2c answer). gorge's affordableTargetCandidates prices that raise per
// candidate, and on a pool-short candidate keeps the menu fail-open while its
// CR 601.2g window census is incomplete -- here the caster's Urza carries an
// unpriced tap-another-artifact mana source (cast_pricing.go's census-gap
// arm). The kernel's free-cast menu withheld the taxed candidates outright,
// so the two engines offered different target menus (dr-mbc-baaa pair 37,
// decision 307).
func TestMBCGorillaShamanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_target_raise_census_open")
}
