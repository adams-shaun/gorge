package oracle

import "testing"

// Invert (instant, {U}{R}): the pair of engines must offer the same castface
// at priority. Replays the tail-39-c000 pair 5 game that parted at decision
// 491 with only gorge offering castface(Invert).
func TestMBCInvertReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "invert_cast_offer")
}
