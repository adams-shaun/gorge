package oracle

import "testing"

// CR 106.1a: Decree of Justice's cycled trigger asks the payer to announce an X
// for "you may pay {X}". gorge bounds that announcement with the payment-aware
// potential pool (rules cumulative.go triggeredCostXAsk, potentialManaFor): a
// "Add {U} or one mana of the chosen colour" source makes exactly its listed
// faces, where the plain PotentialMana over-bound prices it unbounded. The
// kernel's announcement used the plain potential, so a chosen-colour source
// opened X at the 64 cap (decision 611 of 611, dr-mbc-bbaa pair 43) instead of
// the five values gorge offers.
func TestMBCMerfolkSeastalkersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_x_bound_payment_potential")
}
