package oracle

import "testing"

// A-Riveteers Initiate's {1}{B/G} activation: the pay choice offered at the
// activation must be the same in both engines (tail-11-c017 pair 12, decision 612).
func TestMBCARiveteersInitiateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "riveteers_initiate_hybrid_activation_pay")
}
