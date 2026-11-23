package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// improviseAloneRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed
// 3, pod 150): Kappa Cannoneer ({5}{U}, Improvise) with no untapped artifact
// beside Sungrass Prairie ({1},{T}: {G}{W}), whose fee another land funds. The
// improvise credit is empty, so gorge's planner prices the cast plainly, finds
// the census short of the filter's net mana and its prefix search taps the
// filter first (CR 702.126a, 601.2g): the cast is offered. The kernel treated
// any improvise card as a convoke-shaped cast gorge's potential planner
// refuses, and withheld it.
const improviseAloneRepro = "improvise_alone_filter_land_four_seat_pod"

// improviseAloneAt is the decision the recording diverged at.
const improviseAloneAt = 1495

func TestFourSeatImproviseAloneFilterLandCast(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, improviseAloneRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= improviseAloneAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, improviseAloneAt)
	}
}

// TestFourSeatImproviseAloneFilterLandCastMutation proves the replay reaches
// the offer: withholding the improvise-alone cast (the old behaviour) diverges.
func TestFourSeatImproviseAloneFilterLandCastMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugImproviseAloneUnlowered = true
	defer func() { dirac.DebugImproviseAloneUnlowered = false }()
	if res := fourSeatEdhReplay(t, improviseAloneRepro); res.Div == nil {
		t.Fatal("withholding the cast changed no game: the replay does not reach the offer")
	}
}
