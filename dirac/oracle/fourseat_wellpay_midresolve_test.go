package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// wellPayMidResolveRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4
// -seed 3, pod 135): Well of Lost Dreams' "pay {X}" unless-switched cost, priced
// by the filter-land payment search from inside the trigger's resolution, with
// a tapped source whose Taps trigger the step queued. A resolution is no
// priority node to run that trigger from (CR 603.3): the kernel stepped it and
// panicked "bad resolution ask".
const wellPayMidResolveRepro = "well_of_lost_dreams_pay_x_filter_probe_mid_resolution_four_seat_pod"

// wellPayMidResolveAt is the decision the recording panicked at.
const wellPayMidResolveAt = 3293

func TestFourSeatWellPayFilterProbeMidResolution(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, wellPayMidResolveRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= wellPayMidResolveAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, wellPayMidResolveAt)
	}
}

// TestFourSeatWellPayFilterProbeMidResolutionMutation proves the replay reaches
// the probe: settling the queued trigger inside the resolution diverges.
func TestFourSeatWellPayFilterProbeMidResolutionMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugFiltSettleMidResolve = true
	defer func() { dirac.DebugFiltSettleMidResolve = false }()
	if res := fourSeatEdhReplay(t, wellPayMidResolveRepro); res.Div == nil {
		t.Fatal("settling mid-resolution changed no game: the replay does not reach the probe")
	}
}
