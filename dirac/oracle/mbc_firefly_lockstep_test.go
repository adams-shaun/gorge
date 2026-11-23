package oracle

import "testing"

// The p1 priority decision offers casts of Firefly and Gilded Assault Cart and
// an activation of Sorcerer of the Fang on one engine only (tail-51-c000 pair
// 11, decision 283).
func TestMBCFireflyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "firefly_cast_offer")
}
