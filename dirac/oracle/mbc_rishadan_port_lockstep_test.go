package oracle

import "testing"

// The recording is the first divergence of dr-mbc-aaab pair 95 (autopay,
// mulligans 0): decision 540, an upkeep "sacrifice this artifact unless you
// pay" modes ask for p0 after Rishadan Port activations, where gorge offered
// two modes and the kernel one.
func TestMBCRishadanPortReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_upkeep_trigger_restricted_mana")
}
