package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR702KeywordFalseGapsAreRegistered: Renown, Shroud, Phasing,
// Vanishing, Rebound, Compleated and Casualty are implemented (each has a
// behaviour test), so their kw: heads must be registered and a printed
// carrier whose only primitive gap was that head must now be fully
// supported.
func TestCR702KeywordFalseGapsAreRegistered(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, tc := range []struct{ head, card string }{
		{"kw:Renown", "Constable of the Realm"},
		{"kw:Shroud", "Argothian Enchantress"},
		{"kw:Phasing", "Breezekeeper"},
		{"kw:Vanishing", "Deep Forest Hermit"},
		{"kw:Rebound", "Distortion Strike"},
		{"kw:Compleated", "Ajani, Sleeper Agent"},
		{"kw:Casualty", "Cut Your Losses"},
	} {
		if !sup[tc.head] {
			t.Errorf("CR 702 %s: head not registered (carrier %s)", tc.head, tc.card)
		}
		c, ok := reg.Lookup(tc.card)
		if !ok {
			t.Fatalf("CR 702 %s: %s missing from the corpus", tc.head, tc.card)
		}
		var missing []string
		carries := false
		for _, p := range c.Primitives() {
			if p == tc.head {
				carries = true
			}
			if !sup[p] {
				missing = append(missing, p)
			}
		}
		if !carries {
			t.Fatalf("CR 702 %s: precondition: %s does not carry the head", tc.head, tc.card)
		}
		if len(missing) != 0 {
			t.Errorf("CR 702 %s: %s still unsupported, missing %v", tc.head, tc.card, missing)
		}
	}
}
