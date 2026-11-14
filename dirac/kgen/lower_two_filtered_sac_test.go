package kgen

import (
	"strings"
	"testing"
)

// TestTwoFilteredSacrificePartsRefuse: a cost with two filtered sacrifice parts
// (Jarad, Golgari Lich Lord's "Sac<1/Swamp> Sac<1/Forest>") has one sacrifice
// slot in the lowered cost, so the second part overwrote the first and the
// kernel offered the return with no Swamp to sacrifice, which gorge's cost
// payer never does (CR 118.3, 602.2b). A lone filtered part still lowers.
func TestTwoFilteredSacrificePartsRefuse(t *testing.T) {
	const act = "Name:Sac Probe\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ %s | LifeAmount$ 1\n"
	probe := func(cost string) string {
		return strings.Join(lowerScript(t, "Sac Probe", strings.Replace(act, "%s", cost, 1)), "; ")
	}
	if got := probe("Sac<1/Forest>"); got != "" {
		t.Errorf("a lone Sac<1/Forest> refused: %q", got)
	}
	for _, cost := range []string{"Sac<1/Swamp> Sac<1/Forest>", "Sac<1/Forest> Sac<1/Swamp>", "1 Sac<1/Creature> Sac<1/Land>"} {
		if got := probe(cost); !strings.Contains(got, "beside a second sacrifice part") {
			t.Errorf("Cost$ %s: refusals %q, want the second-sacrifice-part refusal", cost, got)
		}
	}
}
