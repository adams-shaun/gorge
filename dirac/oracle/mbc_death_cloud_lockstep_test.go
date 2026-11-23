package oracle

import "testing"

// Death Cloud ({X}{B}{B}{B} sorcery): gorge offered cast(Death Cloud) at a
// priority decision where dirac did not (decision 506, tail-49 pair 14).
func TestMBCDeathCloudReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "death_cloud_x_spell_offer")
}
