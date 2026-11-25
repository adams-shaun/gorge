package oracle

import (
	"strings"
	"testing"
)

// Quickling's enters trigger ("sacrifice it unless you return another creature
// you control to its owner's hand") asks its controller to pay a Return cost
// during resolution. Returning a commander raises the CR 903.9b commander_zone
// ask mid-payment; the resolving trigger must stay on the stack until that
// answer and the rest of the body finish (CR 608.2n). Four-seat edh pod 146
// diverged at decision 1126, where the kernel had already popped the trigger.
const quicklingCmdReturnRepro = "edh_quickling_commander_return"

// quicklingCmdReturnFirstAt is the decision the recording diverged at before
// the fix.
const quicklingCmdReturnFirstAt = 1126

// TestEdhQuicklingCommanderReturnKeepsTriggerLockstep replays the recording
// past the commander_zone decision with the trigger still on the stack.
func TestEdhQuicklingCommanderReturnKeepsTriggerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, quicklingCmdReturnRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Errorf("the kernel stalled: %q", res.Stall)
	}
	if res.Decisions <= quicklingCmdReturnFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, quicklingCmdReturnFirstAt)
	}
}
