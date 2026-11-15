package oracle

import "testing"

// CR 608.2g: a player resolving "Unless [cost]" may activate mana abilities
// before paying, and Remote Farm's "{T}, Remove a depletion counter from
// CARDNAME: Add {W}{W}" (CR 605.3b) is such an ability: its fixed,
// source-anchored SubCounter fee settles with no ask and spends no mana, so
// the unless-pay offer gate must count it among the sources the payment
// window can tap. gorge's census left it out, and with the pool empty it
// offered seat 0 only "Don't pay" for Rhystic Syphon's Unless target player
// pays {3} (decision 813 of tail-51 pair 40) while the kernel offered Pay;
// seat 0 held the farm, a Plains and a Dwarven Mine untapped and could pay.
func TestMBCUnlessOfferCountsDepletionFeeSourceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_offer_counts_depletion_fee_source")
}
