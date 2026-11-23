package oracle

import "testing"

// Vision, Spectral Synthezoid's once-a-turn free may-play cast of the delve
// Temporal Trespass ({X}{U}{U}{U}) under Grand Arbiter Augustin IV's {1} over
// an empty pool: the ten graveyard cards delve the raise away, so gorge's
// priority options include cast(#33, -, 16) while the kernel offers only pass
// and lands (decision 731 of the recorded game, kfuzz tail-52 pair 5 seed
// 913693). The kernel's raise gate did not price the spell's own Delve, and
// wrongly withheld the cast.
//
// The same game also pins the hand permission's pool-only gate under the walk
// bits: two decisions on, the free may-play cast of Artificer's Epiphany (no
// Delve) under the same {1} over an empty pool, gorge offers pass only while
// the kernel's potential-walk probes listed the cast (decision 837, same
// recording). The early accept on the walk bits is a planned-zone one; a hand
// cast falls through to windowPoolPays, which the empty pool fails.
func TestMBCTemporalTrespassReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trespass_cast_offer")
	stdReproNamed(t, "epiphany_hand_walk_gate")
}
