package cards

import (
	"sort"
	"testing"
)

// TestEveryKeywordExpanderIsIdempotent expands one line of every registered
// keyword head twice on the same face and requires the second pass to add
// nothing. A re-expanding head makes a card's compiled value depend on how
// many Link passes it went through, which is what a subset image (decoded,
// then finished again) exposes against the full registry: Transfigure's
// missing has("A", k) guard gave Fleshwrither three activations there and one
// in the full registry (TestImageCorpusOracle).
func TestEveryKeywordExpanderIsIdempotent(t *testing.T) {
	heads := make([]string, 0, len(kwExpanders))
	for h := range kwExpanders {
		heads = append(heads, h)
	}
	sort.Strings(heads)
	for _, h := range heads {
		f := &Face{Name: "Probe", Types: []string{"Creature"}, Keywords: []string{h + ":2"}}
		f.expandKeywords()
		tr, re, ab, st := len(f.Triggers), len(f.Repls), len(f.Abilities), len(f.Statics)
		f.expandKeywords()
		if len(f.Triggers) != tr || len(f.Repls) != re || len(f.Abilities) != ab || len(f.Statics) != st {
			t.Errorf("%s: second expansion added entries: triggers %d->%d repls %d->%d abilities %d->%d statics %d->%d",
				h, tr, len(f.Triggers), re, len(f.Repls), ab, len(f.Abilities), st, len(f.Statics))
		}
	}
}
