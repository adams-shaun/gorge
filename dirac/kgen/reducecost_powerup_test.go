package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lowerSynthRefusals lowers one synthetic card body (the lines after PT:) and
// returns its refusals.
func lowerSynthRefusals(t *testing.T, body string) []refusal {
	t.Helper()
	src := "Name:Reduce Probe\nManaCost:1 R\nTypes:Creature Human\nPT:1/1\n" + body + "\n"
	card, diags := cards.ParseBytes("reduce_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Reduce Probe"}, byName: map[string]*cards.Card{"Reduce Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	return lw.cards[0].Refusals
}

// TestReduceCostPowerUpLowers: a ReduceCost static whose ValidSpell$ is
// Activated.PowerUp lowers to the Power-up-only StatReduceAbility bit, so
// Hulk, Gamma Goliath's "power-up abilities of other creatures you control
// cost {3} less" reaches the kernel. A static of a different activation class
// is still refused by name.
func TestReduceCostPowerUpLowers(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Hulk, Gamma Goliath"})
	if n := len(lw.cards); n != 1 || lw.cards[0].Name != "Hulk, Gamma Goliath" {
		t.Fatalf("lowered %d cards, want Hulk, Gamma Goliath: %v", n, lw.cards)
	}
	c := lw.cards[0]
	if len(c.Refusals) != 0 {
		t.Fatalf("Hulk, Gamma Goliath refused: %v", c.Refusals)
	}
	var red *lStatic
	for i := range c.Statics {
		if c.Statics[i].Kind == "ReduceCost" {
			red = &c.Statics[i]
		}
	}
	if red == nil {
		t.Fatal("Hulk, Gamma Goliath has no lowered ReduceCost static")
	}
	if red.AbilityRed != 4 {
		t.Fatalf("ReduceCost arm %d, want 4 (Power-up only)", red.AbilityRed)
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range bs[0].def.Abilities {
		a := &bs[0].def.Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatReduceAbility {
			continue
		}
		found = true
		if a.Static.B&kop.ReduceAbPowerUp == 0 {
			t.Errorf("StatReduceAbility B %#x lacks ReduceAbPowerUp", a.Static.B)
		}
		if a.Static.B&1 != 0 || a.Static.B&kop.ReduceAbUnlock != 0 {
			t.Errorf("StatReduceAbility B %#x sets another arm", a.Static.B)
		}
	}
	if !found {
		t.Fatal("the built card has no StatReduceAbility static")
	}

	// The same shape naming another activation class is not a Power-up
	// reduction: it refuses, naming the class.
	rs := lowerSynthRefusals(t, "S:Mode$ ReduceCost | Type$ Ability | ValidCard$ Creature.Other+YouCtrl | ValidSpell$ Activated.Boast | Amount$ 3 | AffectedZone$ Battlefield")
	var all []string
	for _, r := range rs {
		all = append(all, r.Reason)
	}
	if got := strings.Join(all, " | "); !strings.Contains(got, `ReduceCost ValidSpell "Activated.Boast"`) {
		t.Fatalf("Activated.Boast refusals %q, want one naming the class", got)
	}
}
