package oracle

import "testing"

// CR 601.2g, 605.3b: Eldritch Immunity's overload cost {4}{C} is payable from
// Forest, two Swamps, a Mage-Ring Network holding a storage counter and a
// Boros Signet (Forest pays the Signet's {1}). The counter makes the
// Network's SubCounter mana ability a census gap that turns gorge's census
// unrelaxable, and gorge then dropped the Signet's fee from the prefix
// search's funding, so its potential walk priced no overloaded cast while the
// kernel offered it (tail-55 autopay pair 5, decision 170 of 170).
func TestMBCEldritchImmunityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overload_signet_fee_funding_lost")
}
