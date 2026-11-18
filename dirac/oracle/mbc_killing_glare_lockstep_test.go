package oracle

import "testing"

// Killing Glare ({X}{B} instant, "destroy target creature with power X or
// less") is offered on both engines or neither at priority (tail-51 pair 16 at
// land e4b1c8471f, decision 202).
func TestMBCKillingGlareReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "killing_glare_cast_offer")
}
