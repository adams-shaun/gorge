package oracle

import "testing"

// CR 601.2b/601.2g: Shattering Blow's {R/W} pip is announced before its mana
// is paid, and the cast is the only legal artifact target's own Treasure. The
// offer is priced by gorge's potentialWitnessKeeping replan (the {W} colouring
// keeps the Treasure; mbc_hybrid_witness_keeping pins the offer). The lowered
// cast's witness must tap that same keeping plan: the kernel's plain witness
// resolved the pip to {R}, sacrificed the Treasure and paid the {1} with the
// Island, while gorge tapped the Plains for the {W} face and the Island for
// the {1}, keeping the target. This recording is the fresh tail-07 focus pair
// 5 (seed 2209069445192442226) at its first divergence, decision 331.
func TestMBCShatteringBlowPaymentWindowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "shattering_blow_payment_window")
}
