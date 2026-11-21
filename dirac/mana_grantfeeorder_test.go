package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestGrantFeeOrderFindsTheGrantLine: a transformed Mystic Skull gives every
// land "{T}: Add one mana of any color", and p0's three untapped lands are a
// Rimewood Falls (printed {U} and {G}), an Island and a Desolate Mire ({1},
// {T}: Add {W}{B}), in that battlefield order. Mijae Djinn's {R}{R}{R} is paid
// by the three grants alone, but gorge's plain prefix search spends its 160
// nodes on the printed colours' subtrees first; its fee-order run (the funding,
// the multi-unit fee gap, then the rest) finds the line and offers the cast
// (CR 605.3a, 601.2g). With the Mire first the plain search finds it.
func TestGrantFeeOrderFindsTheGrantLine(t *testing.T) {
	deck := []cardset.Entry{{Name: "Mystic Skull", Count: 1}, {Name: "Desolate Mire", Count: 1}, {Name: "Rimewood Falls", Count: 1},
		{Name: "Mijae Djinn", Count: 1}, {Name: "Island", Count: 20}}
	tbl, _, _, err := cardsettest.Synth(t, "tail-75").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, lands := range [][]string{
		{"Rimewood Falls", "Island", "Desolate Mire"},
		{"Rimewood Falls", "Desolate Mire", "Island"},
		{"Desolate Mire", "Island", "Rimewood Falls"},
	} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(id["face:Mystic Skull#1"], 0)
		for _, n := range lands {
			o := sc.perm(id[n], 0)
			sc.s.perms[sc.slot(o)].flags &^= pfTapped // (Rimewood Falls enters tapped)
		}
		if !offersCast(sc.s, sc.hand(id["Mijae Djinn"], 0)) {
			t.Errorf("lands %v: Mijae Djinn not offered, want offered (three granted {R})", lands)
		}
	}
}
