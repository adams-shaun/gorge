package oracle

import "testing"

// Saheeli, Radiant Creator paid with many Treasures after the object table
// filled: gorge's planner orders a plan's sources by object id (creation
// order, pay.CompareSteps), while the kernel's ids are reused once MaxObj is
// full, so the raw ids of newly created Treasures sorted before older ones.
// The kernel sacrificed the reused ids 400-402; gorge sacrificed the older
// #551-553 and kept 400-402 (four-seat edh pod of the kfuzz sweep, decision
// 2647). The recording's pool is one-off; the test replays it on the edh
// tables, where no glob helper reaches a four-deck pod.
const saheeliTreasureRepro = "treasure_payment_creation_order_after_object_reuse"

// saheeliTreasureFirstAt is the decision the recording diverged at before the
// fix.
const saheeliTreasureFirstAt = 2647

// TestEdhSaheeliTreasurePaymentLockstep: no divergence and the game goes past
// the recorded decision.
func TestEdhSaheeliTreasurePaymentLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, saheeliTreasureRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= saheeliTreasureFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, saheeliTreasureFirstAt)
	}
}
