package oracle

import "testing"

// CR 602.2, 118.3: Toggo's Rock token gives the creature it equips "{1}, {T},
// Sacrifice Rock: this creature deals 2 damage to any target". Gorge offered
// that activation to the Gurmag Angler the Rock equips at decision 720 of
// dr-mbc-bbab pair 68 and the kernel did not.
func TestMBCGurmagAnglerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_sac_host_activation_needs_mana")
}
