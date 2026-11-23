package oracle

import "testing"

// tail-34-c000 pair 107 (seed 851339): after p0 cast Elminster's Simulacrum
// ("up to one target creature each opponent controls"), gorge offered done on
// the empty target set beside the four creatures while dirac offered only the
// four targets.
func TestMBCElminsterSSimulacrumReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_elminsters_simulacrum_done")
}
