package oracle

import "testing"

// Niall Silvain's {G}{G}{G}{G},{T} regenerate ability at a priority decision:
// gorge and dirac once disagreed on whether p1 was offered
// activate(Niall Silvain) (decision 633 of tail-26 pair 66, seed 754911,
// autopay surface, one mulligan). Both engines now agree; the recording keeps
// them agreeing.
func TestMBCNiallSilvainReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "niall_silvain_priority_offer")
}
