package oracle

import "testing"

// Lady Zhurong, Warrior Queen: the kernel offered a cast of her at decision 444
// that gorge didn't (tail-51-c000, autopay, pair 7).
func TestMBCLadyZhurongWarriorQueenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lady_zhurong_cast_offer")
}
