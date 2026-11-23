package oracle

import (
	"strings"
	"testing"
)

// multikickReductionRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4
// -seed 3, pod 292): Joraga Warcaller ({G}, Multikicker {1}{G}) cast with
// Emerald Medallion in play (green spells cost {1} less). The cast is announced
// with one payment: the total {1}{G}{G} loses its generic to the reduction and
// two Forests pay {G}{G} (CR 601.2f: the reduction applies to the total with the
// additional cost in it). The kernel's witness took the reduction off the base
// alone, found no generic to reduce, and priced three mana: autopay found no
// plan, a panic.
const multikickReductionRepro = "multikicker_reduction_after_payment_witness_four_seat_pod"

// multikickReductionAt is the decision the recording panicked at.
const multikickReductionAt = 939

func TestFourSeatMultikickerReductionAfterPayment(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, multikickReductionRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stalled: %q", res.Stall)
	}
	if res.Decisions <= multikickReductionAt {
		t.Fatalf("the game ended at decision %d, not past %d", res.Decisions, multikickReductionAt)
	}
}
