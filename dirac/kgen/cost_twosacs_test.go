package kgen

import (
	"strings"
	"testing"
)

// TestTwoSacrificePartsRefuse: the kernel pays one Sac pick, so a cost with
// two filtered Sac<N/Spec> parts (CR 118.8: each is a separate sacrifice, paid
// in script order) must refuse rather than keep the last one and offer the
// wrong permanents. A sacrifice of the source beside one filtered part stays
// lowered.
func TestTwoSacrificePartsRefuse(t *testing.T) {
	_, rs := kwardpowBuild(t, "Two Sac Probe", "Name:Two Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n"+
		"A:AB$ Draw | Cost$ B Sac<1/Creature> Sac<1/Artifact> | NumCards$ 1\n")
	if all := kpaylifexReasons(rs); !strings.Contains(all, "second sacrifice part") {
		t.Errorf("Two Sac Probe: refusals %q, want the second sacrifice part refusal", all)
	}
	_, rs = kwardpowBuild(t, "Self Sac Probe", "Name:Self Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n"+
		"A:AB$ Draw | Cost$ B Sac<1/CARDNAME> Sac<1/Creature.Other> | NumCards$ 1\n")
	if all := kpaylifexReasons(rs); strings.Contains(all, "second sacrifice part") {
		t.Errorf("Self Sac Probe: refused %q, a self sacrifice plus one filtered part lowers", all)
	}
}
