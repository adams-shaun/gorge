package oracle

import "testing"

// CR 107.3 with CR 601.2b: the X announced for Knickknack Ouphe ({X}{G}) is
// capped by the mana the player can actually produce. gorge offered X of 0 and
// 1 where the kernel also offered 2 (decision 266 of 266, tail-04-c000 pair 2).
func TestMBCKnickknackOupheReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "knickknack_ouphe_x_cap")
}
