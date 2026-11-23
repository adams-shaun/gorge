package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// improviseGoldRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed
// 4, pod 219): Hellkite Igniter ({5}{R}{R}) under Inspiring Statuary's improvise
// with four untapped artifacts, one a Gold token ("Sacrifice this token: Add
// one mana of any color"). gorge's planner withholds only a credited
// artifact's tapping alternatives, so the Gold the credit also taps still pays
// a red pip (CR 605.3a, 702.126a) and the cast is priced and offered; the
// kernel withheld every credited artifact and left the cast one red short.
const improviseGoldRepro = "improvise_credited_gold_still_pays_four_seat_pod"

// improviseGoldAt is the decision the recording diverged at.
const improviseGoldAt = 1781

func TestFourSeatImproviseCreditedGoldStillPays(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, improviseGoldRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= improviseGoldAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, improviseGoldAt)
	}
}

// TestFourSeatImproviseCreditedGoldStillPaysMutation proves the replay reaches
// the offer: withholding every credited artifact (the old behaviour) diverges.
func TestFourSeatImproviseCreditedGoldStillPaysMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugImproviseWithholdAll = true
	defer func() { dirac.DebugImproviseWithholdAll = false }()
	if res := fourSeatEdhReplay(t, improviseGoldRepro); res.Div == nil {
		t.Fatal("withholding every credited artifact changed no game: the replay does not reach the offer")
	}
}
