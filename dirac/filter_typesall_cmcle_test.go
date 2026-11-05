package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTypesAllCMCLEPredicate: PredTypesAllCMCLE matches an object with EVERY
// type in PredN and a mana value at most PredV.N. Off the battlefield it
// reads the printed types and mana value; on it the derived types and the
// current mana value (s.typesGen and s.cmcPerm).
func TestTypesAllCMCLEPredicate(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "typesall")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := len(tbl.Cards)
	ac2, ac3, ac4 := CardID(n), CardID(n+1), CardID(n+2)
	nc2, a1 := CardID(n+3), CardID(n+4)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Artifact Creature 2", Cost: kop.ManaCost{Generic: 2}, Types: kop.TArtifact | kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Artifact Creature 3", Cost: kop.ManaCost{Generic: 3}, Types: kop.TArtifact | kop.TCreature, Power: 3, Toughness: 3},
		kop.CardDef{Name: "Test Artifact Creature 4", Cost: kop.ManaCost{Generic: 4}, Types: kop.TArtifact | kop.TCreature, Power: 4, Toughness: 4},
		kop.CardDef{Name: "Test Creature 2", Cost: kop.ManaCost{Generic: 2}, Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Artifact 1", Cost: kop.ManaCost{Generic: 1}, Types: kop.TArtifact},
	)
	sc := newScenTableLib(t, tbl, CardID(0), Config{NoElide: true})
	m := kop.TArtifact | kop.TCreature
	fCard := kop.FilterID(len(sc.k.t.Filters))
	sc.k.t.Filters = append(sc.k.t.Filters,
		kop.Filter{Types: m, Zone: kop.ZoneGraveyard, Pred: kop.PredTypesAllCMCLE, PredN: int16(m), PredV: kop.Const(3)},
		kop.Filter{Types: m, Pred: kop.PredTypesAllCMCLE, PredN: int16(m), PredV: kop.Const(3)})
	fPerm := fCard + 1

	cases := []struct {
		name string
		c    CardID
		mv   int
		want bool
	}{
		{"artifact creature mv2", ac2, 2, true},
		{"artifact creature mv3", ac3, 3, true},
		{"artifact creature mv4", ac4, 4, false},
		{"nonartifact creature mv2", nc2, 2, false},
		{"artifact noncreature mv1", a1, 1, false},
	}
	// the precondition: the printed mana values are what the bound is tested
	// against, and the two matching cards differ from the non-matching one
	for _, c := range cases {
		if got := int(sc.k.cards[c.c].total); got != c.mv {
			t.Errorf("%s: printed mana value = %d, want %d", c.name, got, c.mv)
		}
	}
	// the card path reads the printed types and printed mana value, with the
	// card where the target filter looks (its owner's graveyard)
	for _, c := range cases {
		o := sc.obj(c.c, 0)
		sc.s.place(o, kop.ZoneGraveyard, false)
		if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Fatalf("%s: object is in zone %d, want the graveyard", c.name, z)
		}
		if got := sc.s.matchCardGen(sc.k, fCard, c.c, 0, 0, o); got != c.want {
			t.Errorf("card path %s: got %v want %v", c.name, got, c.want)
		}
	}
	// the battlefield path reads the derived types and the current mana value
	for _, c := range cases {
		o := sc.perm(c.c, 0)
		printed := sc.s.typesGen(sc.k, sc.slot(o))
		if c.want && printed&m != m {
			t.Errorf("%s: battlefield types %#x lack %#x", c.name, printed, m)
		}
		if got := sc.s.matchPerm(sc.k, fPerm, sc.slot(o), 0, NoSlot); got != c.want {
			t.Errorf("battlefield path %s: got %v want %v (mv %d)", c.name, got, c.want, sc.s.cmcPerm(sc.k, sc.slot(o)))
		}
	}
}
