package oracle

import "testing"

// Moment of Silence's one-shot "skip your next combat phase this turn" and
// Aggravated Assault's extra combat followed by an additional main phase, both
// cast in the first main phase. gorge's walk enters the extra combat without
// spending the skip, and when the additional main phase completes it resumes at
// the ordinary combat as a spliced entry (extraPhaseBoundary returns ok), which
// skipPendingStep never touches, so the beginning of combat happens. The kernel
// took the resume as an ordinary walk and spent the skip, jumping to the
// postcombat main phase (decision 570 of tail-40-c000 pair 149, seed 422368).
func TestMBCNephaliaDrownyardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "addphase_resume_oneshot_combat_skip")
}
