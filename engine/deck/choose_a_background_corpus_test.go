package deck_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func frontHasSubtype(c *cards.Card, want string) bool {
	for _, ty := range c.Faces[0].Types {
		if strings.EqualFold(ty, want) {
			return true
		}
	}
	return false
}

// TestIsPartnerPairChooseABackgroundCorpus pins the fourth pair family
// (CR 702.124k) against real corpus cards: carriers pair with legendary
// Background enchantments in either order, and nothing else pairs.
func TestIsPartnerPairChooseABackgroundCorpus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	karlach := corpusCard(t, reg, "Karlach, Fury of Avernus")
	laezel := corpusCard(t, reg, "Lae'zel, Vlaakith's Champion")
	wyll := corpusCard(t, reg, "Wyll, Blade of Frontiers")
	veteran := corpusCard(t, reg, "Veteran Soldier")
	fist := corpusCard(t, reg, "Flaming Fist")
	faceless := corpusCard(t, reg, "Faceless One")
	giada := corpusCard(t, reg, "Giada, Font of Hope")

	for _, c := range []*cards.Card{karlach, laezel, wyll, faceless} {
		if !c.Faces[0].HasKeyword("Choose a Background") {
			t.Fatalf("fixture drift: %s no longer carries Choose a Background", c.Faces[0].Name)
		}
	}
	for _, c := range []*cards.Card{veteran, fist} {
		f := c.Faces[0]
		if !f.IsLegendary() || !f.IsEnchantment() || !frontHasSubtype(c, "Background") {
			t.Fatalf("fixture drift: %s is not a legendary Background enchantment", f.Name)
		}
		if strings.Contains(f.Oracle, "can be your commander") || deck.IsCommanderEligible(c) {
			t.Fatalf("fixture drift: %s is commander-eligible alone", f.Name)
		}
	}
	if f := faceless.Faces[0]; !f.IsLegendary() || !f.IsEnchantment() || !f.IsCreature() || !frontHasSubtype(faceless, "Background") {
		t.Fatal("fixture drift: Faceless One is not a legendary enchantment creature Background")
	}

	for _, p := range [][2]*cards.Card{{karlach, veteran}, {laezel, fist}, {wyll, veteran}, {faceless, veteran}, {faceless, karlach}} {
		if !deck.IsPartnerPair(p[0], p[1]) || !deck.IsPartnerPair(p[1], p[0]) {
			t.Fatalf("IsPartnerPair(%s, %s) must be true in both orders", p[0].Faces[0].Name, p[1].Faces[0].Name)
		}
		if !deck.IsCommanderPair(p[0], p[1]) || !deck.IsCommanderPair(p[1], p[0]) {
			t.Fatalf("IsCommanderPair(%s, %s) must be true in both orders", p[0].Faces[0].Name, p[1].Faces[0].Name)
		}
	}
	for _, p := range [][2]*cards.Card{{veteran, fist}, {karlach, laezel}, {karlach, giada}} {
		if deck.IsPartnerPair(p[0], p[1]) || deck.IsCommanderPair(p[0], p[1]) {
			t.Fatalf("%s + %s must not be a pair", p[0].Faces[0].Name, p[1].Faces[0].Name)
		}
	}

	f := deck.File{Name: "KARLACH", Commanders: []string{"Karlach, Fury of Avernus", "Veteran Soldier"},
		Cards: []deck.Entry{{Name: "Karlach, Fury of Avernus", Count: 1}, {Name: "Veteran Soldier", Count: 1},
			{Name: "Mountain", Count: 49}, {Name: "Plains", Count: 49}}}
	if err := f.ValidateCommander(reg); err != nil {
		t.Fatalf("Karlach + Veteran Soldier deck rejected: %v", err)
	}
}
