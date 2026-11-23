package oracle

import "testing"

// Temporal Trespass ({X}{U}{U} sorcery with delve, from tail-52-c000): with
// enough mana in the pool, gorge's priority options include cast(#33, -, 16)
// while the kernel offers only pass and lands (decision 731 of the recorded
// game, kfuzz tail-52 pair 5 seed 913693). One side wrongly withholds the
// cast.
func TestMBCTemporalTrespassReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trespass_cast_offer")
}
