package oracle

import (
	"strings"
	"testing"
)

// TestMayPlayPast16PermissionsNoStall: the std-mkm game in which Expedited
// Inheritance holds 40 play-from-exile permissions at once replays without the
// kernel stopping at its mayplay capacity (the lockstep row only fails on a
// divergence, and a capacity stop is not one).
func TestMayPlayPast16PermissionsNoStall(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	res := replayStdReproResult(t, reg, "custom", "mkm_expedited_inheritance_past_16_permissions", false)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stopped: %s", res.Stall)
	}
}
