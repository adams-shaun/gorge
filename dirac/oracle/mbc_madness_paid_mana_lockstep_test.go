package oracle

import "testing"

// CR 702.35a with CR 605.3b: Incorrigible Youths discarded into exile with
// madness {2}{R} is cast only when the owner can pay; two untapped lands and an
// Energy Refractor ("{2}: add one mana of any color") net three mana only if the
// Refractor's fee is ignored. Gorge's potential-mana bound once ignored it and
// offered the cast the kernel (rightly) did not (tail-02 pair 45 at decision 640; the loop found it as tail-52 pair 88).
func TestMBCIncorrigibleYouthsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_offer_counts_a_paid_mana_ability_net_of_its_fee")
}
