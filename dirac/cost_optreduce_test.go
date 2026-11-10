package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// optReduceScen is Bite Down on Crime in p0's hand with a creature on each
// side, 6+ mana value in p0's graveyard to collect evidence from, and n untapped
// Forests for p0.
func optReduceScen(t *testing.T, forests int) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Bite Down on Crime", Count: 1}, {Name: "Volcanic Dragon", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Forest", Count: 20}}
	set := cardsettest.Set(t, "optcost")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	sc.grave(id["Volcanic Dragon"], 0, 2) // mana value 12 to collect evidence 6
	for i := 0; i < forests; i++ {
		sc.perm(id["Forest"], 0)
	}
	return sc, sc.hand(id["Bite Down on Crime"], 0)
}

func optReduceOffers(s *State, o Obj) (plain, collected bool) {
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
			if a.Mode() == 1 {
				collected = true
			} else {
				plain = true
			}
		}
	}
	return
}

// TestOptCostReduce (CR 601.2b, 601.2f): Bite Down on Crime's "costs {2} less
// if evidence was collected" prices only the cast that collects evidence, at
// the offer and at the charge. Two Forests pay {1}{G} but not {3}{G}, so only
// the collected cast is offered; four pay both, and the plain cast is charged
// {3}{G}.
func TestOptCostReduce(t *testing.T) {
	sc, o := optReduceScen(t, 2)
	if k := sc.k; k.gen == nil || !k.gen.anyOptCost || !k.bargainRed || !optCostCard(k, sc.s.objs[o].card) {
		t.Fatal("precondition: Bite Down is not an OptionalCost card with a priced reduction")
	}
	if plain, collected := optReduceOffers(sc.s, o); plain || !collected {
		t.Errorf("two Forests: plain cast offered %v, collected cast offered %v; want false and true", plain, collected)
	}

	sc, o = optReduceScen(t, 4)
	if plain, collected := optReduceOffers(sc.s, o); !plain || !collected {
		t.Fatalf("four Forests: plain cast offered %v, collected cast offered %v; want both", plain, collected)
	}
	for _, tc := range []struct {
		name    string
		mode    uint8
		generic uint8
	}{{"plain cast", 0, 3}, {"collected cast", 1, 1}} {
		sc, o = optReduceScen(t, 4)
		sc.s.Apply(MakeAction(AkCast, uint8(o), 0, tc.mode))
		if sc.s.ctl != ctlBuild {
			t.Fatalf("%s: control %d, want the cast build", tc.name, sc.s.ctl)
		}
		if got := sc.s.buildCost(sc.k); got.Generic != tc.generic {
			t.Errorf("%s: charged generic %d, want %d", tc.name, got.Generic, tc.generic)
		}
	}
}
