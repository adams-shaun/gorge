package oracle

import (
	"strings"
	"testing"
)

// TestKaitoFifthEmblemNoStall: the game that once stopped on the kernel's
// capacity when Kaito, Bane of Nightmares made a fifth emblem now plays out
// its script and 200 decisions past it, in agreement with gorge.
func TestKaitoFifthEmblemNoStall(t *testing.T) {
	reg := synthReg(t)
	res := replayStdReproResult(t, reg, "custom", "dsk_kaito_fifth_emblem", false)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stopped: %q", res.Stall)
	}
}
