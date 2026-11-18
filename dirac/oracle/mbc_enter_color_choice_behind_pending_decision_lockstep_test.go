package oracle

import (
	"testing"
)

// CR 614.12 with gorge's one-at-a-time entry path: Battle at the Helvault's
// departure batch returns four exiled cards at once. Its controller's Gift of
// Granite (a returning Aura) parks the CR 303.4f host ask, and Jeweled
// Torque's "As Jeweled Torque enters, choose a color" reaches the entry
// boundary while that ask is still outstanding. gorge's one-at-a-time rule
// queues only the Aura choice behind another decision; every other as-enters
// ask keeps the pre-existing path, so the entry happens unasked and the
// recorded colour is the deterministic first colour of the option list ("W").
// The kernel queued the colour ask behind the parked host ask and posed it as
// the next decision instead (decision kind ref priority p1, kernel choose p1;
// dr-mbc pair 35 before MBC was split).
func TestMBCAtzocanSeerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "enter_color_choice_behind_pending_decision")
}
