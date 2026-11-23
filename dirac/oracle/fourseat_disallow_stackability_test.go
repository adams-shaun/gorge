package oracle

import (
	"strings"
	"testing"
)

// disallowStackAbilityRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4
// -seed 4, pod 275): Disallow aimed at a stack ability while Jubilant Skybonder
// (a target-gated cost raise, CR 601.2f) prices each candidate. The ability's
// target code lies beyond the object table; the kernel indexed it (panic).
const disallowStackAbilityRepro = "disallow_targets_stack_ability_cost_filter_four_seat_pod"

// disallowStackAbilityAt is the decision the recording panicked at.
const disallowStackAbilityAt = 1920

func TestFourSeatDisallowStackAbilityCostFilter(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, disallowStackAbilityRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= disallowStackAbilityAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, disallowStackAbilityAt)
	}
}
