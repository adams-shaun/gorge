package oracle

import "testing"

// std-msh's repros, on the same terms as std_set_repros_lockstep_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-msh")
}
