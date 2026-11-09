package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

func hybridLower(face *cards.Face) *lCard {
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestHybridSecondaryCosts: a hybrid pip in a flashback or sneak mana part is
// lowered and encoded like a printed cost's (the kernel announces each pip's
// colour in the cast); a resolution mana cost keeps refusing it, because the
// kernel's resolution payment holds only generic and coloured pips.
func TestHybridSecondaryCosts(t *testing.T) {
	wb := kop.ColorMask(kop.ColW | kop.ColB)
	ur := kop.ColorMask(kop.ColU | kop.ColR)

	t.Run("sneak", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "4 WB WB", Types: []string{"Creature"}, Keywords: []string{"Sneak:3 WB"}})
		if len(c.Refusals) != 0 || c.Sneak == nil {
			t.Fatalf("sneak with a hybrid pip: refusals %v, sneak %v", c.Refusals, c.Sneak)
		}
		if len(c.Sneak.Hybrid) != 1 || c.Sneak.Generic != 3 {
			t.Fatalf("sneak cost %s, want {3}{W/B}", c.Sneak)
		}
		m, err := manaCostOf(*c.Sneak)
		if err != nil || m.Generic != 3 || m.Hyb[0] != wb || m.Hyb[1] != 0 {
			t.Errorf("encoded sneak cost %+v (%v), want generic 3 and one W/B pip", m, err)
		}
	})

	t.Run("flashback with a discard", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "1 UR", Types: []string{"Instant"}, Keywords: []string{"Flashback:1 UR UR Discard<1/Card>"}})
		if len(c.Refusals) != 0 || c.Flashback == nil || c.FbCost == nil {
			t.Fatalf("flashback with hybrid pips and a discard: refusals %v", c.Refusals)
		}
		m, err := manaCostOf(*c.Flashback)
		if err != nil || m.Generic != 1 || m.Hyb[0] != ur || m.Hyb[1] != ur || m.Hyb[2] != 0 {
			t.Errorf("encoded flashback mana %+v (%v), want generic 1 and two U/R pips", m, err)
		}
		if c.FbCost.DiscardN != 1 {
			t.Errorf("flashback non-mana part %+v, want a discard of one card", *c.FbCost)
		}
	})

	t.Run("flashback mana only", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "R", Types: []string{"Instant"}, Keywords: []string{"Flashback:2 UR"}})
		if len(c.Refusals) != 0 || c.Flashback == nil || c.FbCost != nil || len(c.Flashback.Hybrid) != 1 {
			t.Fatalf("flashback {2}{U/R}: refusals %v, mana %v, fbcost %v", c.Refusals, c.Flashback, c.FbCost)
		}
	})

	t.Run("flashback discard shapes still refused", func(t *testing.T) {
		for _, kw := range []string{"Flashback:1 UR Discard<1/Card> Sac<1/Creature>", "Flashback:1 UR Discard<1/Creature>"} {
			c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "R", Types: []string{"Instant"}, Keywords: []string{kw}})
			if !hasRefusal(c.Refusals, "flashback cost") {
				t.Errorf("%s: refusals %v, want a flashback cost refusal", kw, c.Refusals)
			}
		}
	})

	t.Run("resolution mana cost", func(t *testing.T) {
		c := hybridLower(&cards.Face{
			Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"},
			Abilities: []*cards.SA{{Kind: "SP", API: "Draw", Params: map[string]string{
				"Defined": "You", "NumCards": "1", "UnlessCost": "WB", "UnlessPayer": "You",
			}}},
		})
		if !hasRefusal(c.Refusals, "resolution mana cost") {
			t.Errorf("hybrid resolution cost: refusals %v, want the resolution mana cost refusal", c.Refusals)
		}
	})
}
