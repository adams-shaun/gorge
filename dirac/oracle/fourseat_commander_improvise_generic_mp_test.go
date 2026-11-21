package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// commanderImproviseGenericRepro is a four-seat edh pod (kfuzz -kernel edh
// -seats 4 -seed 4, pod 122): Gimbal, Gremlin Prodigy cast from the command zone
// at {2}{G}{U}{R} plus a {2} tax, with Inspiring Statuary giving it improvise
// and three untapped artifacts. gorge's offer credits the printed generic with
// those artifacts (CR 702.126a, before the CR 903.8 tax) and offers the cast;
// the kernel's command-zone offer priced the cast with no credit and withheld
// it when the lands alone fell short.
const commanderImproviseGenericRepro = "commander_improvise_credit_generic_four_seat_pod"

// commanderImproviseGenericAt is the decision the recording diverged at.
const commanderImproviseGenericAt = 971

func TestFourSeatCommanderImproviseCreditsGeneric(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, commanderImproviseGenericRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= commanderImproviseGenericAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, commanderImproviseGenericAt)
	}
}

// TestFourSeatCommanderImproviseCreditsGenericMutation proves the replay
// reaches the offer: pricing the command-zone cast with no credit diverges.
func TestFourSeatCommanderImproviseCreditsGenericMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCommanderNoCredit = true
	defer func() { dirac.DebugCommanderNoCredit = false }()
	if res := fourSeatEdhReplay(t, commanderImproviseGenericRepro); res.Div == nil {
		t.Fatal("an uncredited offer changed no game: the replay does not reach it")
	}
}
