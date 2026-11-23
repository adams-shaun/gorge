package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// filterReflectRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 5,
// pod 279): Sungrass Prairie's all-at-once {1},{T}: Add {G}{W} under Mirari's
// Wake ("whenever you tap a land for mana, add one mana of any type that land
// produced", CR 106.12a), ahead of Shalai's {4}{G}{G}. gorge's reflected pick is a
// mid-ability ask it cannot pose, so it keeps the first candidate ({W}); the
// kernel added no extra for the filter's tap, so one {G} fewer was left floating.
const filterReflectRepro = "each_filter_reflect_extra_mirari_wake_sungrass_prairie_four_seat_pod"

// filterReflectAt is the decision the recording diverged at.
const filterReflectAt = 4629

func TestFourSeatEachFilterReflectExtra(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, filterReflectRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= filterReflectAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, filterReflectAt)
	}
}

// TestFourSeatEachFilterReflectExtraMutation proves the replay reaches the
// filter's tap: dropping its reflected extra diverges.
func TestFourSeatEachFilterReflectExtraMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEachReflectNone = true
	defer func() { dirac.DebugEachReflectNone = false }()
	if res := fourSeatEdhReplay(t, filterReflectRepro); res.Div == nil {
		t.Fatal("dropping the extra changed no game: the replay does not reach the filter's tap")
	}
}
