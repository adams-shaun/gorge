package oracle

import "testing"

// Awaken the Woods ({X}{G}{G}): the announced-X choice offers the same X range
// on both engines (tail-08 pair 14, decision 80).
func TestMBCAwakenTheWoodsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_spell_choose_one_more")
}
