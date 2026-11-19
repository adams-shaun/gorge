package oracle

import "testing"

// Monkey Cage's "When a creature enters, sacrifice Monkey Cage and create X
// 2/2 green Monkey creature tokens, where X is that creature's mana value":
// the trigger sacrifices with RememberSacrificed$ True before its
// TriggeredCard$CardManaCost read, and gorge's count read TriggeredCard as the
// whole Remembered list, so X summed the Cage's own mana value (5) with the
// entering creature's. Dawning Angel (mana value 5) made 10 Monkeys in gorge
// and 5 in the kernel (tail-75 pair 2, decision 297, NInst); Emperor
// Crocodile (4) made 9 and 4 (pair 1, decision 448). gorge now reads the
// trigger's capture (effects.triggeredCardRemembered).
func TestMBCMonkeyCageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "monkey_cage_triggered_card_mv")
}

func TestMBCEmperorCrocodileReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "monkey_cage_emperor_crocodile_mv")
}
