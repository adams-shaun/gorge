package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// wakeFilterRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 4,
// pod 220): Trostani, Selesnya's Voice ({G}{G}{W}{W}) under Mirari's Wake with a
// Forest, Sungrass Prairie ({1},{T}: Add {G}{W}) and Selesnya Signet. Tapping the
// Prairie makes {G}{W} and the Wake adds one more mana of a type the land
// produced (CR 106.12a, 605.3b); gorge keeps the first candidate, {W}, so the
// Forest's {G}{G} pays the fee and the Prairie plus the extra make the second {W}
// and gorge's prefix search funds the cast. The kernel priced no extra for the
// all-at-once filter and never offered the cast; filtExtra now adds the first
// colour's extra, so the pod plays clean with no declared stall.
const wakeFilterRepro = "wake_pick_extra_on_filter_land_four_seat_pod"

func TestFourSeatWakePickExtraOnFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, wakeFilterRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("stall %q, want a clean game", res.Stall)
	}
}

// TestFourSeatWakePickExtraOnFilterMutation proves the replay reaches the
// filter's extra: dropping it diverges.
func TestFourSeatWakePickExtraOnFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEachReflectNone = true
	defer func() { dirac.DebugEachReflectNone = false }()
	if res := fourSeatEdhReplay(t, wakeFilterRepro); res.Div == nil {
		t.Fatal("dropping the extra changed no game: the replay does not reach the filter step")
	}
}
