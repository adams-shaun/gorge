package oracle

import "testing"

// Restless Bones' "target creature gains swampwalk" offered General Tazri to
// gorge but Tazri and two tokens to dirac (decision 723 of tail-12-c001 pair 26).
func TestMBCRestlessBonesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restless_bones_target_tokens")
}
