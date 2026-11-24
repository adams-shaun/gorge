package oracle

import "testing"

// CR 614.1, 510.2: Avatar of Slaughter (double strike for every creature) is
// blocked by Zephyr Sentinel and Ioreth of the Healing House under its
// controller's Lashknife Barrier. Ioreth's 1 damage becomes 0 and Zephyr's 2
// becomes 1; the kernel read the zero as a prevention of everything dealt to
// Avatar and dropped Zephyr's damage too (decision 1093, std tail-55 pair 7:
// gorge marks 1, the kernel 0).
func TestMBCAvatarOfSlaughterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combat_damage_rewritten_to_zero_keeps_other_events")
}
