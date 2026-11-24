package oracle

import "testing"

// CR 601.2g, 605.3b: Eldritch Immunity's overload cost {4}{C} looks payable
// from Forest, two Swamps, a Mage-Ring Network holding a storage counter and a
// Boros Signet (Forest pays the Signet's {1}). The counter makes the Network's
// SubCounter mana ability a gap with no relaxed alternative, so gorge's
// potential census is unrelaxable, the Signet's fee is no funding and its
// prefix search prices no overloaded cast. The kernel treated the Network's
// plain first unit as pricing the battery (storeGapLive) and funded the fee
// anyway (tail-55 autopay pair 5, decision 170 of 170).
func TestMBCEldritchImmunityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overload_signet_fee_funding_lost")
}
