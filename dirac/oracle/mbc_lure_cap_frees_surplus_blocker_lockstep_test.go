package oracle

import "testing"

// CR 509.1c with CR 509.1b: the equipped Bristling Boar's lure ("all
// creatures able to block it do so") makes both able defenders duty blockers,
// but the Boar's own can't-be-blocked-by-more-than-one ceiling hosts only one
// of them, so the surplus duty blocker is free and its plain pairs stay
// offered (Reckless Fireweaver and Thrull Parasite against the attacking
// creature tokens; dr-mbc-baab pair 153, decision 706). The kernel once
// offered the two duty blockers only their lure blocks.
func TestMBCRecklessFireweaverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lure_cap_frees_surplus_blocker")
}
