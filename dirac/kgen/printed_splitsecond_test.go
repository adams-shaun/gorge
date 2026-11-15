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

// A token's own token is lowered with the pool: Diabolical Salvation's Devil
// makes a Treasure as it dies, and no other card of the pool makes one, so
// the Devil's build would find no Treasure in the pool and refuse the card.
func TestPoolLowersTheTokenATokenMakes(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const name, devil, treasure = "Diabolical Salvation", "r_4_4_devil_haste_treasure", "c_a_treasure_sac"
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
	lw := lowerPool(p)
	keys := map[string]*lCard{}
	for _, tk := range lw.tokens {
		keys[tk.Name] = tk
	}
	d := keys[devil]
	if d == nil || len(d.Tokens) != 1 || d.Tokens[0] != treasure {
		t.Fatalf("the pool's tokens %v: want %s making %s", keys, devil, treasure)
	}
	if keys[treasure] == nil {
		t.Fatalf("the pool's tokens %v lack %s, the token %s makes", keys, treasure, devil)
	}
}
