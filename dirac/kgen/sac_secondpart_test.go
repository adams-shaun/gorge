package kgen

import (
	"strings"
	"testing"
)

// A second plain Sac<> cost part used to overwrite the first (Jarad, Golgari
// Lich Lord asked one sacrifice where gorge asks two). Beside a mana cost it
// refuses; without one a printed activated ability lowers it as a cost
// program (TestTwoFilteredSacrificePartsRefuse).
func TestSecondPlainSacPartRefuses(t *testing.T) {
	const act = "Name:Two Sac Probe\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ %s | LifeAmount$ 1\n"
	if rs := exileBfSynth(t, strings.Replace(act, "%s", "Sac<1/Artifact>", 1)); len(rs) != 0 {
		t.Fatalf("a single Sac<1/Artifact> refused: %q", keclReason(rs))
	}
	_, rs, _ := costProgBuild(t, "Two Sac Probe", strings.Replace(act, "%s", "1 Sac<1/Artifact> Sac<1/Creature>", 1))
	got := keclReason(rs)
	if !strings.Contains(got, "beside a second sacrifice part") {
		t.Errorf("two plain Sac parts: refusals %q, want the second-sacrifice refusal", got)
	}
}
