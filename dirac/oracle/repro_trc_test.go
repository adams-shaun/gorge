package oracle

import "testing"

// std-trc's games that once diverged at the end of their script, on the
// autopay surface: both engines now agree there, and for 200 decisions after.
func TestStdTrcReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-trc")
}
