package oracle

import "testing"

func TestMBCStealArtifactReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gate_x_mana_with_no_other_gates")
}
