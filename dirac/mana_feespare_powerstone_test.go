package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 106.6, 605.3a: a Powerstone's {C} can't pay a nonartifact spell but does
// pay the {2} of Urn of Godfire's "{2}: Add one mana of any color", an activated
// mana ability, and that mana then pays Grim Physician's {B}. gorge's bound
// counts it (potentialFeeSpare); the kernel's potential walk priced the fee
// against the spell's own admitted pool and dropped the cast.
func TestFeeSparePowerstoneFundsUrnForSpell(t *testing.T) {
	set := cardsettest.Synth(t, "tail-50")
	deck := []cardset.Entry{{Name: "Plains", Count: 10}}
	for _, n := range []string{"Urn of Godfire", "Grim Physician", "Mox Sapphire", "A-Urza's Command"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the tail-50 table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for i := range tbl.Cards { // (a token def has no name)
		if c := &tbl.Cards[i]; c.Types&kop.TToken != 0 && c.Types&kop.TArtifact != 0 && len(c.Abilities) == 1 && c.Abilities[0].Kind == kop.AbMana && c.Abilities[0].ManaRestrict != 0 {
			id["Powerstone Token"] = CardID(i)
		}
	}
	if _, ok := id["Powerstone Token"]; !ok {
		t.Fatal("precondition: no Powerstone token in the table")
	}
	for _, c := range []struct {
		board []string
		want  bool
	}{
		{[]string{"Plains", "Plains"}, true}, // the control: unrestricted mana pays the fee
		{[]string{"Powerstone Token", "Plains"}, true},
		{[]string{"Powerstone Token", "Powerstone Token"}, true},
		{[]string{"Powerstone Token", "Mox Sapphire"}, true},
		{[]string{"Powerstone Token"}, false}, // one {C} leaves the {2} fee short
	} {
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		for _, n := range c.board {
			sc.perm(id[n], 0)
		}
		sc.perm(id["Urn of Godfire"], 0)
		gp := sc.hand(id["Grim Physician"], 0)
		d := sc.s.Step()
		if got := castOffered(&d, gp); got != c.want {
			t.Errorf("board %v + Urn: Grim Physician offered=%v, want %v", c.board, got, c.want)
		}
	}
}
