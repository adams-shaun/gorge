package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// commandZoneConvokeOfferRepro is a four-seat hosted-cmdr pod (kfuzz -kernel
// hosted-cmdr -seats 4 -policy heuristic, pod 177): Kasla, the Broken Halo
// (printed convoke) in the command zone. Convoke functions on the stack
// whatever zone the spell was cast from (CR 702.51a), so the command-zone cast
// is offered on the untapped creatures' credit like a hand cast. gorge priced
// the printed cost with no credit and withheld it, while the kernel offered it
// from a pool that the credit completed.
const commandZoneConvokeOfferRepro = "command_zone_convoke_commander_offered_four_seat_pod"

// commandZoneConvokeOfferAt is the decision the recording diverged at, and
// commandZoneConvokeCreditAt the first command-zone offer in it that only the
// credit makes.
const (
	commandZoneConvokeOfferAt  = 1259
	commandZoneConvokeCreditAt = 1109
)

func TestHostedCmdrCommandZoneConvokeOfferLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := hostedCmdrPodReplay(t, commandZoneConvokeOfferRepro, func(*GorgeSide) {})
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= commandZoneConvokeOfferAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, commandZoneConvokeOfferAt)
	}
	// The precondition: the replay reaches an offer that needs the credit.
	dirac.DebugCommanderNoCredit = true
	defer func() { dirac.DebugCommanderNoCredit = false }()
	res = hostedCmdrPodReplay(t, commandZoneConvokeOfferRepro, func(*GorgeSide) {})
	if res.Div == nil || res.Div.At != commandZoneConvokeCreditAt {
		t.Fatalf("an uncredited kernel offer did not diverge at %d: %+v", commandZoneConvokeCreditAt, res.Div)
	}
}
