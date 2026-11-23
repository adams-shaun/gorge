package oracle

import "testing"

// The priority menu after the opponent-visible Urza resolves must offer the
// same Augury Adept cast in both engines (tail-53 pair 5, decision 584).
func TestMBCAuguryAdeptReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "augury_adept_cast_offer")
}
