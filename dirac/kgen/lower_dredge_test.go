package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Printed Dredge N (CR 702.52) lowers to CardDef.Dredge = N: every corpus
// card whose only blocker was the keyword lowers with no refusal and its
// printed number.
func TestDredgeLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	want := map[string]uint8{
		"Dakmor Salvage": 2, "Darkblast": 3, "Golgari Grave-Troll": 6, "Golgari Thug": 4,
		"Grave-Shell Scarab": 1, "Greater Mossdog": 3, "Life from the Loam": 3, "Moldervine Cloak": 2,
		"Necroplasm": 2, "Nightmare Void": 2, "Shambling Shell": 3, "Shenanigans": 1, "Stinkweed Imp": 5,
	}
	for name, n := range want {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		if _, ok := card.Faces[0].KeywordParam("Dredge"); !ok {
			t.Fatalf("%s: no printed Dredge keyword", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		c := lowerCard(p, card.Faces[0].Name, card, false)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		if c.Dredge != n {
			t.Errorf("%s: Dredge = %d, want %d", name, c.Dredge, n)
		}
	}
}

// A Dredge number outside 1..15 refuses by name.
func TestDredgeRefusesBadNumber(t *testing.T) {
	for _, param := range []string{"0", "16", "X", ""} {
		lc := &lowerCtx{out: &lCard{}}
		lc.lowerDredge(param)
		if len(lc.out.Refusals) == 0 || lc.out.Dredge != 0 {
			t.Errorf("Dredge:%q lowered (Dredge=%d, refusals %v), want a refusal", param, lc.out.Dredge, lc.out.Refusals)
		}
	}
}
