package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// splitSecondCards print K:Split second (CR 702.61a); each lowers to
// CardDef.SplitSecond, the flag the kernel's split second gate reads off a
// spell on the stack (dirac/kw_splitsecond.go).
var splitSecondCards = []string{
	"Celestial Crusader", "Diabolical Salvation", "In Too Deep", "Krosan Grip",
	"Siege Smash", "Stonewood Invocation", "Sudden Death", "Sudden Edict",
	"Sudden Shock", "Sudden Spoiling", "Sulfur Elemental", "Take Possession",
	"Wipe Away", "Word of Seizing",
}

func TestPrintedSplitSecondLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range splitSecondCards {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		hasKw := false
		for _, k := range card.Faces[0].Keywords {
			hasKw = hasKw || cards.KeywordHead(k) == "Split second"
		}
		if !hasKw {
			t.Fatalf("%s prints no K:Split second", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		c := lowerCard(p, card.Faces[0].Name, card, false)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if !c.SplitSecond {
			t.Errorf("%s: CardDef.SplitSecond not set", name)
		}
		for _, tk := range c.Tokens {
			tc, _ := reg.Token(tk)
			if tl := lowerCard(p, tk, tc, true); len(tl.Refusals) != 0 {
				t.Errorf("%s token %s refusals = %v, want none", name, tk, tl.Refusals)
			}
		}
	}
}
