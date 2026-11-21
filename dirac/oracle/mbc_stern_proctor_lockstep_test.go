package oracle

import "testing"

// Stern Proctor ({U}{U}) is offered by gorge at decision 213 of the recorded
// game (tail-50-s000, pair 25) while dirac leaves it out of p1's priority
// options. The recording replays both engines clean once they agree.
func TestMBCSternProctorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stern_proctor_options")
}
