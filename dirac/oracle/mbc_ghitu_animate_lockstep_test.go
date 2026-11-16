package oracle

import "testing"

// CR 605.3a, 602.2b: Ghitu Encampment's {1}{R} animate is payable from a
// repeatable fee-only filter (Orochi Leafcaller's {G}: add one mana of any
// color) beside untapped Forests, with Ghitu itself already tapped. gorge
// offered the animate at the priority decision (decision 158 of tail-57 pair
// 7289); the kernel left it out.
func TestMBCGhituEncampmentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ghitu_animate_filter_repeat")
}
