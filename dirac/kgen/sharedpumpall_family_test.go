package kgen

import (
	"strings"
	"testing"
)

// TestSharedPumpAllFamilySpellings: gorge's SharedKeywordSpellings treats
// Hexproof and Trample as family words and grants the sharing card's own
// spelling ("Hexproof:Black", "Trample:Planeswalker"), which the kernel has
// no grant for. The lowering refuses instead of granting the bare keyword,
// and the refusal names the spelling. Odric lists Hexproof and Trample;
// Concerted Effort lists Trample only.
func TestSharedPumpAllFamilySpellings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		card     string
		spelling string
		refused  []string
		lowered  []string
	}{
		{
			name:     "hexproof-from",
			card:     "Knight of Grace",
			spelling: "Hexproof:Black",
			refused:  []string{"Odric, Lunarch Marshal"},
			lowered:  []string{"Concerted Effort"},
		},
		{
			name:     "trample-over",
			card:     "Kevin, Questing Dragon",
			spelling: "Trample:Planeswalker",
			refused:  []string{"Odric, Lunarch Marshal", "Concerted Effort"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lw := lowerNamedPool(t, []string{"Odric, Lunarch Marshal", "Concerted Effort", "Grizzly Bears", tc.card})
			byName := map[string]*lCard{}
			for _, c := range lw.cards {
				byName[c.Name] = c
			}
			for _, n := range tc.refused {
				c := byName[n]
				if c == nil {
					t.Fatalf("%s not lowered", n)
				}
				found := false
				for _, r := range c.Refusals {
					if strings.Contains(r.Reason, tc.spelling) {
						found = true
					}
				}
				if !found {
					t.Fatalf("%s refusal does not name %q: %v", n, tc.spelling, c.Refusals)
				}
			}
			for _, n := range tc.lowered {
				c := byName[n]
				if c == nil {
					t.Fatalf("%s not lowered", n)
				}
				if len(c.Refusals) != 0 {
					t.Fatalf("%s refused: %v", n, c.Refusals)
				}
			}
		})
	}
}

// TestWithHexproofStaysRefused: gorge's filtered with<Keyword> words go
// through Face.HasKeyword, which compares by keyword head, so withHexproof
// also matches Hexproof:<spec>. The kernel's plain KwHexproof bit does not,
// so the word stays refused; withFear (a keyword with no parameterised
// spelling) still lowers.
func TestWithHexproofStaysRefused(t *testing.T) {
	if _, err := parseFilter("Creature.withHexproof", func(string) bool { return false }, func(string) bool { return false }); err == nil {
		t.Fatal("withHexproof parsed: gorge also matches Hexproof:<spec> by head, which the kernel cannot read")
	}
	if _, err := parseFilter("Creature.withFear", func(string) bool { return false }, func(string) bool { return false }); err != nil {
		t.Fatalf("withFear stopped parsing: %v", err)
	}
}
