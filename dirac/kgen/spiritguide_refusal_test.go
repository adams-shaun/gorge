package kgen

import (
	"strings"
	"testing"
)

// TestSpiritGuideHandManaRefusal: a mana ability that functions from the hand
// and exiles its own card (the Spirit Guides) is refused by name with the
// reason the kernel cannot model it, while the other ExileFromHand<1/CARDNAME>
// shapes (a non-mana hand ability) keep their own, older refusal.
func TestSpiritGuideHandManaRefusal(t *testing.T) {
	const want = "a hand-zone mana ability: the kernel has no hand source in its mana window"
	cs := lowerNamed(t, []string{"Simian Spirit Guide", "Elvish Spirit Guide", "Masked Bandits"})
	seen := map[string]bool{}
	for _, c := range cs {
		seen[c.Name] = true
		var got string
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "exile cost") {
				got = r.Reason
			}
		}
		spirit := strings.HasSuffix(c.Name, "Spirit Guide")
		if spirit && !strings.Contains(got, want) {
			t.Errorf("%s: refusals %v, want the hand-zone mana reason %q", c.Name, c.Refusals, want)
		}
		if !spirit && strings.Contains(got, want) {
			t.Errorf("%s: a non-mana hand ability took the mana reason: %q", c.Name, got)
		}
	}
	for _, n := range []string{"Simian Spirit Guide", "Elvish Spirit Guide", "Masked Bandits"} {
		if !seen[n] {
			t.Errorf("%s was not lowered at all", n)
		}
	}
}
