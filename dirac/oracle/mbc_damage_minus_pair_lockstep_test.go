package oracle

import "testing"

// CR 616.1: two Lashknife Barriers both rewrite the combat damage dealt to
// their controller's creature, and gorge asks the affected player which
// applies next (decision 818 of tail-55 pair 0, Ethereal Investigator blocked
// by two creatures). The two minus lines commute, so the kernel poses the same
// no-op ask (replOrderCandsSeat) and the recording replays in lockstep; it was
// a declared stall (StallReplPlus) while the kernel dealt the damage without
// the ask. A noncombat pair still stalls (TestReplDmgRewriteClash).
func TestMBCEtherealInvestigatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_minus_pair_order_ask")
}
