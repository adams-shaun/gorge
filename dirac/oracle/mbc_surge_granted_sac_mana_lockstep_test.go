package oracle

import "testing"

// CR 702.117a with CR 601.2g: Grip of the Roil is cast for its surge cost
// ({1}{U}) from an empty pool by sacrificing the tapped Eldrazi Spawn for {1}
// and tapping the untapped one for {U} through Path of Annihilation's granted
// "Eldrazi you control have '{T}: Add one mana of any color.'". The kernel's
// autopay surge probe priced only printed taps, with the granted and
// sacrifice-only sources excluded, so it withheld gorge's surged option
// (decision 237 of 237, dr-mbc-babb pair 61).
func TestMBCGripOfTheRoilReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "surge_granted_sac_mana")
}
