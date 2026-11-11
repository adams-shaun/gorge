package oracle

import (
	"strings"
	"testing"
)

// TestStarcageTenLinksNoStall: Pinnacle Starcage entering over a wide board
// exiles ten artifacts and creatures until it leaves. The kernel once held
// eight such links and stopped on its capacity; the game now plays out its
// script and 200 decisions past it, in agreement with gorge.
func TestStarcageTenLinksNoStall(t *testing.T) {
	res := replayStdReproResult(t, synthReg(t), "std-eoe", "starcage_exiles_ten_until_it_leaves", true) // recorded on the full surface
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stopped: %q", res.Stall)
	}
}
