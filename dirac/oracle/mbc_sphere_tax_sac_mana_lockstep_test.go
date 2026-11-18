package oracle

import "testing"

// CR 601.2f with CR 605.1a: Bonethorn Valesk ({4}{R}) under the opponent's
// Sphere of Resistance costs {4}{R}{1}; three Swamps, an Azorius Signet
// filtered through a fourth Swamp, and a sacrificed Skirk Prospector pay it, so
// the cast is on the priority menu (pair 14 of tail-18-c000, decision 893).
func TestMBCBonethornValeskReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sphere_tax_sac_mana_signet_cast_offer")
}
