package oracle

import (
	"strings"
	"testing"
)

// kessigUnpayableRepro is a four-seat edh pod (kfuzz -kernel edh -surface full,
// pod 77): p2 activates Kessig Wolf Run's {X}{R}{G},{T} with X = 3 and pays by
// hand with Forest, Plains, Arcane Signet and Rogue's Passage, leaving the {R}
// pip with no source able to make red. gorge reverses the activation (CR 733.1,
// the produced mana floats); the kernel's manual window kept going and settled
// the cost.
const kessigUnpayableRepro = "kessig_wolf_run_unpayable_x_reverses_four_seat_pod"

// kessigUnpayableAt is the decision the recording diverged at.
const kessigUnpayableAt = 651

func TestEdhKessigWolfRunUnpayableReversesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, kessigUnpayableRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= kessigUnpayableAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, kessigUnpayableAt)
	}
}
