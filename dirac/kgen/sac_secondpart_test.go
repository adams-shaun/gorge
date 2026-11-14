package kgen

import (
	"strings"
	"testing"
)

// A second plain Sac<> cost part used to overwrite the first (Jarad, Golgari
// Lich Lord asked one sacrifice where gorge asks two); it now refuses.
func TestSecondPlainSacPartRefuses(t *testing.T) {
	const act = "Name:Two Sac Probe\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ %s | LifeAmount$ 1\n"
	if rs := exileBfSynth(t, strings.Replace(act, "%s", "Sac<1/Artifact>", 1)); len(rs) != 0 {
		t.Fatalf("a single Sac<1/Artifact> refused: %q", keclReason(rs))
	}
	got := keclReason(exileBfSynth(t, strings.Replace(act, "%s", "Sac<1/Artifact> Sac<1/Creature>", 1)))
	if !strings.Contains(got, "beside a second sacrifice part") {
		t.Errorf("two plain Sac parts: refusals %q, want the second-sacrifice refusal", got)
	}
}
