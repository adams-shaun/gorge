package oracle

import "testing"

// TestManaModalBackSourcePlanReproLockstep is tail-32 pair 88: a modal DFC
// played as its land back face beside Urborg, Tomb of Yawgmoth (every land is
// also a Swamp), with a printed {T}: Add {R} of its own. Gorge's payment
// planner taps the back face for {B} through the granted Swamp; the kernel
// plans the same {B} from Foul Orchard instead, so the cast's planned payment
// taps a different land and the projection's tapped flags diverge.
func TestManaModalBackSourcePlanReproLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_modal_back_source_plan")
}
