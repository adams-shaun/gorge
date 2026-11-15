package oracle

import "testing"

// Brothers of Fire's activated ability is offered by one engine only at a
// priority decision (tail-06 pair 470, decision 277).
func TestMBCBrothersOfFireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "brothers_of_fire_activation_offer")
}
