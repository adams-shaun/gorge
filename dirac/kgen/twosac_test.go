package kgen

import (
	"strings"
	"testing"
)

// TestTwoPlainSacParts: two plain Sac<> cost parts (Jarad, Golgari Lich Lord's
// Sac<1/Artifact> Sac<1/Creature.Other>) refuse by reason instead of lowering the last
// part alone.
func TestTwoPlainSacParts(t *testing.T) {
	const src = "Name:Two Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\nA:AB$ Draw | Cost$ Sac<1/Artifact> Sac<1/Creature.Other> | NumCards$ 1\n"
	_, rs := kwardpowBuild(t, "Two Sac Probe", src)
	if all := kpaylifexReasons(rs); !strings.Contains(all, "second sacrifice part") {
		t.Errorf("Two Sac Probe: refusals %q, want the second sacrifice part refusal", all)
	}
}
