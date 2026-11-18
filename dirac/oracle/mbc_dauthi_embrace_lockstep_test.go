package oracle

import "testing"

// Dauthi Embrace's {B}{B} activation (CR 602.2, 117.1b) is offered at priority
// on both engines or neither (tail-42-c001 pair 9 at land 5ecb58e7f8,
// decision 436).
func TestMBCDauthiEmbraceActivationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dauthi_embrace_shadow_activation")
}
