package kgen

import (
	"strings"
	"testing"
)

// TestTwoPlainSacParts: two plain Sac<> cost parts with mana beside them
// (Jarad, Golgari Lich Lord's graveyard return pays mana with its two
// sacrifices) refuse by reason instead of lowering the last part alone: the
// planner's reservation is single-part. With no mana, two filtered sacrifices
// lower as per-atom cost-program picks (T18b4). The probe specs are types the
// synthetic probe pool's type vocabulary knows: a bare land type is not.
func TestTwoPlainSacParts(t *testing.T) {
	const src = "Name:Two Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\nA:AB$ Draw | Cost$ 1 Sac<1/Artifact> Sac<1/Creature.Other> | NumCards$ 1\n"
	_, rs := kwardpowBuild(t, "Two Sac Probe", src)
	if all := kpaylifexReasons(rs); !strings.Contains(all, "second sacrifice part") {
		t.Errorf("Two Sac Probe: refusals %q, want the second sacrifice part refusal", all)
	}
}

// TestTwoPlainSacPartsNoManaLower: the no-mana form lowers as a cost program.
func TestTwoPlainSacPartsNoManaLower(t *testing.T) {
	const src = "Name:Two Sac Free Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\nA:AB$ Draw | Cost$ Sac<1/Artifact> Sac<1/Creature.Other> | NumCards$ 1\n"
	_, rs := kwardpowBuild(t, "Two Sac Free Probe", src)
	if all := kpaylifexReasons(rs); strings.Contains(all, "second sacrifice part") {
		t.Errorf("Two Sac Free Probe: refusals %q, want it to lower", all)
	}
}
