package oracle

import "testing"

// Herald's Horn's upkeep trigger may reveal a creature of the chosen type.
// Abrade destroys the Horn in response, so the trigger resolves with the
// Horn's last known information, its chosen type (CR 113.7a, 608.2b, 608.2h):
// the may-reveal ask is still posed. The kernel read no chosen type once the
// source left (srcChosen's departure snapshot) and finished the trigger
// without asking, so its stack was one object short at decision 2179.
func TestCmd5HeraldsHornSourceLeftAtMayReveal(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_heralds_horn_source_left_at_may_reveal", 2179, "NStack")
	requireCmd5ReproClean(t, res, 2179)
}
