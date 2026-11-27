package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAltCostPickGrantLowers: Dream Halls' "Rather than pay the mana cost for a
// spell, its controller may discard a card that shares a color with that
// spell" (ValidSA$ Spell | Cost$ Discard<1/Card.SharesColorWith/...>, no
// ValidPlayer$) lowers as a battlefield StatAlternativeCost grant carrying
// kop.StAltGrant and kop.StAltAnyCaster, its cost the filtered discard pick
// with the dynamic colour test (kop.Cost.DiscardSharesCastColor) and no fixed
// hand filter. It was refused as "AlternativeCost shape".
func TestAltCostPickGrantLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Dream Halls")
	if !ok || card == nil {
		t.Fatal("Dream Halls not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	if r := lowerRefusals(lc); len(r) != 0 {
		t.Fatalf("Dream Halls refused: %v", r)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("Dream Halls build: %v", err)
	}
	n := 0
	for _, a := range all[0].def.Abilities {
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAlternativeCost {
			continue
		}
		n++
		if a.Static.StFlags&kop.StAltGrant == 0 {
			t.Errorf("grant flag unset (StFlags %b)", a.Static.StFlags)
		}
		if a.Static.StFlags&kop.StAltAnyCaster == 0 {
			t.Errorf("any-caster flag unset (StFlags %b): Dream Halls has no ValidPlayer$", a.Static.StFlags)
		}
		if a.Static.Filter != 0 {
			t.Errorf("Static.Filter = %d, want 0 (Dream Halls has no ValidCard$)", a.Static.Filter)
		}
		if a.Cost.Flags != kop.CostDiscard || a.Cost.N != 1 || a.Cost.Mana != (kop.ManaCost{}) {
			t.Errorf("cost = flags %b N %d mana %+v, want CostDiscard/1/zero", a.Cost.Flags, a.Cost.N, a.Cost.Mana)
		}
		if !a.Cost.DiscardSharesCastColor {
			t.Error("cost does not carry DiscardSharesCastColor: the discard filter is not the cast card's colour")
		}
		if a.Cost.Filter != 0 || a.Cost.Discard2N != 0 || a.Cost.DiscardHand || a.Cost.DiscardRandom || a.Cost.DiscardX {
			t.Errorf("cost carries another discard/filter part: %+v", a.Cost)
		}
	}
	if n != 1 {
		t.Fatalf("%d StatAlternativeCost abilities, want 1", n)
	}
}
