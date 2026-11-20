package kgen

import (
	"strings"
	"testing"
)

// TestRestrictedManaCostRefusalNamesThePart pins the gap4-07 carriers'
// refusals: a spend-restricted mana ability whose cost goes beyond a plain {T}
// refuses by naming the cost part (a mana fee, a self-sacrifice, a life
// payment, a counter cost, a sacrifice or tap of another permanent) instead of
// the old two-class reason, and the old exact two classes are gone. No card is
// lowered until the kernel's restricted-cost gap has lockstep evidence
// (AGENTS.md's lockstep rule; the follow-up ticket).
func TestRestrictedManaCostRefusalNamesThePart(t *testing.T) {
	const prefix = "mana restriction on a source whose cost goes beyond {T}: "
	cases := []struct{ name, part string }{
		{"Castle Garenbrig", "a mana fee"},
		{"Cormela, Glamour Thief", "a mana fee"},
		{"Great Hall of the Biblioplex", "a life payment"},
		{"Great Hall of the Citadel", "a mana fee"},
		{"Hydro-Channeler", "a mana fee"},
		{"Interplanar Beacon", "a mana fee"},
		{"Jeweled Lotus", "a self-sacrifice"},
		{"Orb of Dragonkind", "a mana fee"},
		{"Thieving Varmint", "a life payment"},
		{"Voldaren Estate", "a life payment"},
		{"Crucible of the Spirit Dragon", "a counter cost"},
		{"Cryptic Trilobite", "a counter cost"},
		{"Throne of Makindi", "a counter cost"},
		{"Master of Dark Rites", "a sacrifice of another permanent"},
		{"Grand Architect", "a tap of another permanent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lc := lowerCorpusCard(t, c.name)
			var got []string
			for _, rf := range lc.Refusals {
				got = append(got, rf.Reason)
			}
			if len(got) != 1 || !strings.HasPrefix(got[0], prefix) || !strings.Contains(got[0], c.part) {
				t.Fatalf("%s: refusals = %v, want exactly one %q + %q", c.name, got, prefix, c.part)
			}
			for _, old := range []string{
				"mana restriction on a PayLife source: gorge plans it last-resort",
				"mana restriction on a source whose cost goes beyond {T}",
			} {
				if strings.TrimSpace(got[0]) == old {
					t.Fatalf("%s: the old two-class reason %q is back", c.name, old)
				}
			}
		})
	}
}

// TestRestrictedLifeSourcesLower: a sole restricted {T}, Pay N life ability of
// one fixed colour (Gnarlroot Trapper's Elf-only {G}, Untaidake's
// legendary-only {C}{C}) lowers: the kernel plans it as a last-resort unit
// (dirac mana_restrictedcost.go); no refusal names its cost.
func TestRestrictedLifeSourcesLower(t *testing.T) {
	for _, name := range []string{"Gnarlroot Trapper", "Untaidake, the Cloud Keeper"} {
		for _, rf := range lowerCorpusCard(t, name).Refusals {
			t.Errorf("%s: refused: %s", name, rf.Reason)
		}
	}
}
