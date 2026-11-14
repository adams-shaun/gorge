package kgen

import (
	"strings"
	"testing"
)

// TestTwoPlainSacParts: two plain Sac<> cost parts (Jarad, Golgari Lich Lord's
// graveyard return asks Sac<1/Swamp> Sac<1/Forest>) refuse by reason instead of
// lowering the last part alone. The probe specs are types the synthetic probe
// pool's type vocabulary knows: a bare land type is not, so a Sac<1/Swamp>
// Sac<1/Forest> probe refuses at the filter before the second part is read.
func TestTwoPlainSacParts(t *testing.T) {
	const src = "Name:Two Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\nA:AB$ Draw | Cost$ Sac<1/Artifact> Sac<1/Creature.Other> | NumCards$ 1\n"
	_, rs := kwardpowBuild(t, "Two Sac Probe", src)
	if all := kpaylifexReasons(rs); !strings.Contains(all, "second sacrifice part") {
		t.Errorf("Two Sac Probe: refusals %q, want the second sacrifice part refusal", all)
	}
}
