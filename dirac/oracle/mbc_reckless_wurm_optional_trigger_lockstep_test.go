package oracle

import "testing"

// Reckless Wurm's "When you cycle Reckless Wurm, ..." is an optional trigger:
// at the trigger's own decision its controller must be offered both yes and
// no, even when the trigger's action has nothing to do. The kernel once
// offered only no (decision 410; tail-14-c000 pair 34, seed 484207).
func TestMBCRecklessWurmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reckless_wurm_optional_trigger")
}
