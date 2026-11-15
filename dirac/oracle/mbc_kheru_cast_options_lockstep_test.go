package oracle

import "testing"

// Pool tail-55 (autopay, mulligans 0, seed 1) pair 68: the kernel offered
// p0 a Kheru Bloodsucker cast at decision 97 that gorge did not. The recording
// is a custom-pool one: a tail-NN bucket moves whenever the lowered set grows
// (Dauthi Jackal left tail-55), so the table is built from the decks' own cards.
func TestMBCTail55ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kheru_bloodsucker_cast_offered")
}
