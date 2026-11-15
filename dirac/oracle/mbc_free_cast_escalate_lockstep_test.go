package oracle

import "testing"

// CR 118.9d with CR 702.120a: a spell cast without paying its mana cost
// still pays its additional costs, and Escalate is one, due for each mode
// chosen beyond the first. Yahenni's Expertise free-casts Borrowed Malevolence
// (Escalate {2}) with one untapped land left, so only one mode is affordable.
// The gorge side once captured the Escalate parameter only on the hand-cast
// path, so the effect-cast path offered a second mode (and would have charged
// nothing for it) where the kernel went straight to the target
// (decision 191, tail-06 pair 565).
func TestMBCYahenniSExpertiseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_escalate_unaffordable")
}
