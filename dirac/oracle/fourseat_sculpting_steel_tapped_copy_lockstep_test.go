package oracle

import (
	"strings"
	"testing"
)

// sculptingSteelTappedRepro is a four-seat edh pod (kfuzz -kernel edh pod 80):
// Sculpting Steel resolves and enters as a copy of Goldmire Bridge. The copy
// takes the Bridge's self "enters tapped" replacement (CR 614.12, 707.5), so
// the kernel's copy entered tapped; gorge's entered untapped.
const sculptingSteelTappedRepro = "clone_copies_enters_tapped_sculpting_steel_goldmire_bridge_four_seat_pod"

// sculptingSteelTappedAt is the decision the recording diverged at.
const sculptingSteelTappedAt = 981

func TestEdhSculptingSteelTappedCopyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, sculptingSteelTappedRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= sculptingSteelTappedAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, sculptingSteelTappedAt)
	}
}
