package oracle

import "testing"

// CR 601.2c: Tear Asunder ({1}{G}, kicker {1}{B}) exiles a target artifact or
// enchantment; only the kicked cast also announces a target nonland permanent.
// With only lands in play (p0's Forest and artifact land Vault of Whispers,
// p1's Island) the plain cast targets the Vault, and gorge offers it. The
// kernel's offer gate once demanded a candidate for the kicked-only second
// slot too and offered only pass (decision 53 of the recording).
func TestMBCTearAsunderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_only_target_slot_needs_no_candidate")
}
