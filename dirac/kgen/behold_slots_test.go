package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestBeholdCostSlots: one Behold<N/Type> cost reading serves every slot
// that carries it. A flashback cost's behold beside its mana lowers to the
// card's flashback behold (the mana stays the flashback cost, no FbCost); an
// either-or's behold of a planeswalker subtype lowers like a creature
// type's; a behold of a type a ChooseCreatureType cost part names refuses by
// that mechanism; a card type, and a flashback behold beside a non-mana
// part, keep refusing.
func TestBeholdCostSlots(t *testing.T) {
	lower := func(kws []string, statics []cards.Static) (*lCard, string) {
		c := &cards.Card{Faces: []*cards.Face{{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Sorcery"}, Keywords: kws, Statics: statics}}}
		// a Jace in the pool, so the planeswalker subtype is a known word
		jace := &cards.Card{Faces: []*cards.Face{{Name: "Synthetic Jace", ManaCost: "2 U", Types: []string{"Legendary", "Planeswalker", "Jace"}}}}
		p := &pool{byName: map[string]*cards.Card{"Synthetic": c, "Synthetic Jace": jace}, names: []string{"Synthetic", "Synthetic Jace"}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Synthetic", c, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		return lc, strings.Join(got, " | ")
	}
	t.Run("flashback mana and behold three", func(t *testing.T) {
		lc, refused := lower([]string{"Flashback:1 R Behold<3/Elemental>"}, nil)
		if refused != "" {
			t.Fatalf("refused: %s", refused)
		}
		if lc.Flashback == nil || lc.Flashback.String() != "{1R}" {
			t.Fatalf("flashback mana %v, want {1R}", lc.Flashback)
		}
		if lc.FbBeholdN != 3 || lc.FbBeholdF == nil || lc.FbCost != nil {
			t.Fatalf("flashback behold %d of %v, FbCost %v: want 3 Elementals and no FbCost", lc.FbBeholdN, lc.FbBeholdF, lc.FbCost)
		}
		var b strings.Builder
		dumpLIR(&b, lc)
		if !strings.Contains(b.String(), "fbbehold=3") {
			t.Errorf("listing %q has no fbbehold=3", b.String())
		}
	})
	t.Run("either-or behold of a planeswalker type", func(t *testing.T) {
		lc, refused := lower([]string{"AlternateAdditionalCost:Behold<1/Jace>:1"}, nil)
		if refused != "" {
			t.Fatalf("refused: %s", refused)
		}
		if lc.SpellCost == nil || lc.SpellCost.BeholdN != 1 || lc.SpellCost.BeholdF == nil || lc.AltAddCost == nil {
			t.Fatalf("spell cost %+v, alt-add %+v: want a behold of one Jace or the mana", lc.SpellCost, lc.AltAddCost)
		}
	})
	for _, c := range []struct {
		name    string
		kws     []string
		statics []cards.Static
		want    string
	}{
		{"chosen type", nil, []cards.Static{{Mode: "OptionalCost", Params: map[string]string{"Mode": "OptionalCost", "EffectZone": "All", "ValidCard": "Card.Self", "ValidSA": "Spell", "Cost": "ChooseCreatureType<1> Behold<2/Creature.ChosenType>"}}}, "a creature type chosen as a cost part"},
		{"card type", []string{"AlternateAdditionalCost:Behold<1/Artifact>:1"}, nil, `alternate additional cost "Behold<1/Artifact>:1"`},
		{"flashback behold beside a sacrifice", []string{"Flashback:1 R Behold<1/Elemental> Sac<1/Creature>"}, nil, "flashback cost"},
		{"two in an either-or", []string{"AlternateAdditionalCost:Behold<2/Goblin>:1"}, nil, `alternate additional cost "Behold<2/Goblin>:1"`},
	} {
		lc, refused := lower(c.kws, c.statics)
		if !strings.Contains(refused, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, refused, c.want)
		}
		if lc.FbBeholdN != 0 || lc.OptCost != nil || lc.SpellCost != nil && lc.SpellCost.BeholdN != 0 {
			t.Errorf("%s: a refused cost kept its behold", c.name)
		}
	}
}
