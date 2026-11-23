package oracle

import "testing"

// Domesticated Watercourse's {U}{B} animate ability is offered by both
// engines (recorded from tail-49 pair 49, surface autopay, decision 816).
func TestMBCDomesticatedWatercourseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "domesticated_watercourse_animate_offer")
}
