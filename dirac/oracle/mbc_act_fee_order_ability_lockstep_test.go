package oracle

import "testing"

// CR 601.2g, 605.3b: Electric Eel's {1}{R} activation beside a fee-carrying
// filter (the tail-15 fx-wh pool, pair 26). gorge's plain prefix search for
// the activation spends its whole 160-node budget on the fee-carrying gaps
// and its fee-ordered run (potentialPrefixScript's build sets feeOrder for
// optAbility too) then funds the filters' fees from covered sources before
// the gaps. The kernel's feeOrderOf refused every non-cast play, so it never
// walked the fee order and dropped the activation gorge offered.
//
// The recording is a new capture of the shrunk pair-26 scenario, not one of
// the hunt's divergent-pair recordings (its own pair-26 game diverged
// elsewhere); its provenance is the fee-order diagnosis, not a pool replay.
func TestMBCElectricEelFeeOrderAbilityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_fee_order_ability_reopens_budget_spent_offer")
}
