package oracle

import "testing"

func TestMBCFeedbackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "feedback_aura_cast_offer")
}
