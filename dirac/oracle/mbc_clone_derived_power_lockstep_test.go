package oracle

import "testing"

// CR 613.4 with CR 707.2: Deceptive Frostkite's as-enters copy choice
// ("a creature you control with power 4 or greater") reads the creature's
// power as the layers leave it. Snapcaster Mage wearing Vectis Gloves is a
// 4/1, so it is a legal template; gorge matched the printed 2 and left only
// "enter as itself" on the menu (decision 389; ev-tail-40-c000 pair 30).
func TestMBCSnapcasterMageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clone_etb_choice_reads_derived_power")
}
