package oracle

import "testing"

// CR 614.12 / 707.2: Pirated Copy entering as a copy of Flycatcher Giraffid
// enters with the copied "enters with your choice of a vigilance counter or a
// reach counter" replacement, so its controller chooses the counter kind. Gorge
// skipped the choice (its entry fold cannot absorb a several-kind counter body)
// and went straight to priority; the kernel offered choose:counter twice
// (decision 458, tail-54 pair 55 seed 393387).
func TestMBCFlycatcherGiraffidReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clone_copied_counter_kind_choice")
}
