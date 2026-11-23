package oracle

import "testing"

// bf_tgtreduce_target_menu: the ev-tail-34-c000 evidence loop's pair 47
// (dr-mbc, seed 15656988849564238848) parted at decision 503, the target ask
// of seat 0's Numbing Dose ("Enchant artifact or creature"). Seat 0's own
// Killian, Ink Duelist's ReduceCost ("Spells you cast that target a creature
// cost {2} less to cast") made the offer's optimistic probe price the cast at
// 1UU, which the floating pool pays; the kernel's target menu then kept every
// candidate, including the equipped Conjurer's Mantle, an artifact target the
// reduction does not price down to 3UU's five mana with no untapped source
// left to tap. gorge's affordableTargetCandidates reprices per candidate and
// withheld it; the kernel's affordableTargets now does too.
func TestMBCConjurerSMantleTargetMenuReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bf_tgtreduce_target_menu")
}
