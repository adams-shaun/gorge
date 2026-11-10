package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestVariableAltCosts: an alternative or turn-up cost that carries an {X} or
// a printed reduction. A warp cost's {X} is announced like a printed one (the
// kernel's X ask prices the replaced cost); a face-down keyword's second colon
// field is a generic reduction of its turn-up cost (CR 601.2f, 702.168c); a
// waterbend cost of a variable amount keeps refusing (the kernel's waterbend
// credit reads a fixed count, and X can't be 0 there).
func TestVariableAltCosts(t *testing.T) {
	t.Run("warp X", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "X G G", Types: []string{"Creature"}, Keywords: []string{"Warp:X G"}})
		if len(c.Refusals) != 0 || c.Warp == nil {
			t.Fatalf("warp {X}{G}: refusals %v, warp %v", c.Refusals, c.Warp)
		}
		m, err := manaCostOf(*c.Warp)
		if err != nil || m.X != 1 || m.Generic != 0 || m.Pips[4] != 1 {
			t.Errorf("encoded warp cost %+v (%v), want one {X} and one {G}", m, err)
		}
	})

	t.Run("warp XX still refused", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "G", Types: []string{"Creature"}, Keywords: []string{"Warp:X X G"}})
		if !hasRefusal(c.Refusals, "warp cost") {
			t.Errorf("warp {X}{X}{G}: refusals %v, want a warp cost refusal", c.Refusals)
		}
	})

	t.Run("disguise reduced by a graveyard count", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Creature"},
			Keywords: []string{"Disguise:5 R:N:a reduction"},
			SVars:    map[string]string{"N": "Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn"}})
		if len(c.Refusals) != 0 || c.MorphCost == nil {
			t.Fatalf("disguise with a reduction: refusals %v", c.Refusals)
		}
		if c.MorphCost.Generic != 5 || c.MorphReduce.Kind != vCountGraveyard {
			t.Errorf("turn-up cost %s reduced by %s, want {5}{R} less a graveyard count", c.MorphCost, c.MorphReduce)
		}
	})

	t.Run("plain disguise has no reduction", func(t *testing.T) {
		c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Creature"}, Keywords: []string{"Disguise:5 R"}})
		if len(c.Refusals) != 0 || c.MorphCost == nil || c.MorphReduce.Kind != vNone {
			t.Fatalf("plain disguise: refusals %v, reduction %s", c.Refusals, c.MorphReduce)
		}
	})

	t.Run("disguise reduction shapes still refused", func(t *testing.T) {
		for _, kw := range []string{"Disguise:5 R:N", "Disguise:5 R:Missing:text", "Disguise:R:N:text", "Disguise:X R:N:text"} {
			c := hybridLower(&cards.Face{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Creature"}, Keywords: []string{kw},
				SVars: map[string]string{"N": "Count$ValidGraveyard Instant.YouOwn"}})
			if !hasRefusal(c.Refusals, "disguise cost") {
				t.Errorf("%s: refusals %v, want a disguise cost refusal", kw, c.Refusals)
			}
		}
	})

	t.Run("waterbend X beside printed mana still refused", func(t *testing.T) {
		c := hybridLower(&cards.Face{
			Name: "Synthetic", ManaCost: "2 W", Types: []string{"Creature"},
			Abilities: []*cards.SA{{Kind: "AB", API: "Draw", Params: map[string]string{
				"Cost": "1 Waterbend<X>", "XMin": "1", "Defined": "You", "NumCards": "1",
			}}},
			SVars: map[string]string{"X": "Count$xPaid"},
		})
		if !hasRefusal(c.Refusals, "waterbend cost") {
			t.Errorf("waterbend {X}: refusals %v, want the waterbend cost refusal", c.Refusals)
		}
	})
}
