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

// commanderConvokeGrantRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4
// -seed 1, pair 30): Olivia, Opulent Outlaw in the command zone under We Ride at
// Dawn ("Legendary creature spells you cast have convoke") with five untapped
// creatures and no untapped land. gorge's command-zone gate prices the printed
// cost with no convoke credit and withholds the cast; the improvise credit above
// reached the kernel's offer for a convoke grant too and offered it (decision 2118).
const commanderConvokeGrantRepro = "commander_convoke_grant_command_zone_no_credit_four_seat_pod"

// commanderConvokeGrantAt is the decision the recording diverged at.
const commanderConvokeGrantAt = 2118

func TestFourSeatCommanderConvokeGrantNotCredited(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, commanderConvokeGrantRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= commanderConvokeGrantAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, commanderConvokeGrantAt)
	}
}
