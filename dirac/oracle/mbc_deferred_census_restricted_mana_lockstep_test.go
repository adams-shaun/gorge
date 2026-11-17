package oracle

import "testing"

// Flamekin Brawler's "{R}: this creature gets +1/+0 until end of turn" is a
// mana ability-free activated ability, so it belongs in the priority menu
// whenever its cost is payable. The recorded game has gorge offering
// activate(#6{Flamekin Brawler},-,0) at decision 186 where dirac offered only
// pass, which is the divergence this recording pins.
func TestMBCFlamekinBrawlerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "deferred_census_restricted_mana")
}
