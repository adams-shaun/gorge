package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// beaconRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 5, pod
// 129): Terminate ({B}{R}) beside The Beacon of Kahless, whose Battlefront
// ability makes N mana of ONE chosen colour (CR 106.1; N the opponents
// attacked). gorge's potential plan never pays two different pips from one tap
// of it; the kernel priced its wide view as N independent any-colour units and
// offered the cast.
const beaconRepro = "beacon_battlefront_one_colour_four_seat_pod"

// beaconAt is the decision the recording diverged at.
const beaconAt = 970

func TestFourSeatBeaconBattlefrontOneColour(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, beaconRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= beaconAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, beaconAt)
	}
}

// TestFourSeatBeaconBattlefrontOneColourMutation proves the replay reaches the
// offer: pricing the wide view as independent units diverges.
func TestFourSeatBeaconBattlefrontOneColourMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDynPairIndependent = true
	defer func() { dirac.DebugDynPairIndependent = false }()
	if res := fourSeatEdhReplay(t, beaconRepro); res.Div == nil {
		t.Fatal("independent units changed no game: the replay does not reach the offer")
	}
}
