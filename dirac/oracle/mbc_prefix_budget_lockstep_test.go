package oracle

import "testing"

// Treasure Trove's second {2}{U}{U} activation after Rain of Filth ("lands you
// control gain 'Sacrifice this land: Add {B}'") with The World Tree granting
// every land any-colour mana: gorge's prefix search over the ten land gaps runs
// out of its 164-clone budget walking sacrifice branches (potentialPrefixScript,
// plain run limited) and offers only pass, while dirac's search finds the
// two-tap line (decision 841, tail-08 pair 12).
func TestMBCTreasureTroveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prefix_budget_land_sac_grants")
}
