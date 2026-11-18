package oracle

import "testing"

// Peema Outrider ({2}{G}{G}) in hand beside three untapped Swamps under a
// tapped Chromatic Lantern and an untapped Jaspera Sentinel: the Lantern's
// static ("Lands you control have '{T}: Add one mana of any color'") still
// grants while its source is tapped (CR 611.3c), so {2}{G}{G} is payable
// through the granted lands and Jaspera's tap-another ability, and gorge
// offers the cast at decision 382 (pool tail-133, mulligans 1, surface
// full); the kernel's grant search found that prefix and still left the
// cast off the offers (grantCap kept only the repriced and store-step
// search successes).
func TestMBCPeemaOutriderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_paid_cast_not_offered")
}
