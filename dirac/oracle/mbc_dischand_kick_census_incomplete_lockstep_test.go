package oracle

import (
	"testing"
)

// A multikicked hand cast whose one-payment total the caster's printed sources
// alone cannot pay, and whose only further source is a Lion's Eye Diamond:
// gorge's payment census leaves each Diamond a gap (its activation carries a
// discard-hand part the planner cannot execute), and the prefix search over
// that gap never completes for a hand play -- the activation's cost discards
// the spell it would fund. gorge's verdict for the multikicked Enclave Elite is
// therefore census_incomplete with no script (tail-03 pair 55, decision 74) and
// the autopay surface drops it; the kernel must not offer the kicked cast
// beside the plain one both engines keep out (the card's optional-cost shape).
func TestMBCEnclaveEliteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dischand_kick_census_incomplete")
}
