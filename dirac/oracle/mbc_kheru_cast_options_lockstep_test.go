package oracle

import "testing"

// Pool tail-55 (autopay, mulligans 0, seed 1) pair 68: the kernel offered
// p0 a Kheru Bloodsucker cast at decision 97 that gorge did not.
func TestMBCTail55ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kheru_bloodsucker_cast_offered")
}
