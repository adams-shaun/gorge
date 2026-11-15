package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// delveScen is a Hill Giant ({3}{R}) in hand over a Mountain and an
// Island with three Opt in the graveyard; withTeval puts Teval, Arbiter of
// Virtue (the granted delve) on the battlefield.
func delveScen(t *testing.T, withTeval bool) (*scen, Obj, []Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Teval, Arbiter of Virtue", Count: 2},
		{Name: "Hill Giant", Count: 2},
		{Name: "Opt", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Island", Count: 4},
	}
	set := cardsettest.Set(t, "stackkwdelve")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Island"], 0)
	if withTeval {
		sc.perm(id["Teval, Arbiter of Virtue"], 0)
	}
	grave := sc.grave(id["Opt"], 0, 3)
	giant := sc.hand(id["Hill Giant"], 0)
	// preconditions: a graveyard to delve, a generic cost the two lands cannot pay
	if sc.s.pl[0].graveN != 3 || sc.k.t.Cards[id["Hill Giant"]].Cost.Generic != 3 {
		t.Fatalf("graveN %d, giant generic %d", sc.s.pl[0].graveN, sc.k.t.Cards[id["Hill Giant"]].Cost.Generic)
	}
	return sc, giant, grave
}

func TestDelveGrantCast(t *testing.T) {
	sc, giant, grave := delveScen(t, true)
	if casts := sc.castOptions(giant); len(casts) != 1 {
		t.Fatalf("with Teval the Giant is offered %v, want the one delve-credited cast", casts)
	}
	sc.castOption(t, giant, AkCast, 0)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenDelve {
		t.Fatalf("first build decision %v/%d, want the delve ask", d.Kind, d.Sub)
	}
	if d.N != 4 { // the three graveyard cards and Done
		t.Fatalf("delve ask has %d options, want 3 cards and Done: %v", d.N, d.Options[:d.N])
	}
	// the options are the three graveyard cards (in any order) and Done
	var picks []Action
	done := false
	for i := 0; i < int(d.N); i++ {
		switch a := d.Options[i]; {
		case a.Kind() == AkDone:
			done = true
		case a.Kind() == AkChoose && a.Source() == TCode(uint8(grave[len(picks)])):
			picks = append(picks, a)
		default:
			t.Fatalf("option %d = kind %v src %d, want a graveyard card of %v or Done", i, a.Kind(), a.Source(), grave)
		}
	}
	if !done || len(picks) != 3 {
		t.Fatalf("options %v: done %v, picks %d", d.Options[:d.N], done, len(picks))
	}
	for i := 0; i < 3; i++ {
		d = sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenDelve {
			t.Fatalf("pick %d: decision %v/%d", i, d.Kind, d.Sub)
		}
		sc.s.Apply(picks[i])
	}
	if d = sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision after the picks %v/%d, want priority with the Giant on the stack", d.Kind, d.Sub)
	}
	for _, o := range grave {
		if z := sc.s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("delved card %d in zone %v, want exile", o, z)
		}
	}
	if sc.s.pl[0].graveN != 0 {
		t.Fatalf("graveyard holds %d cards, want 0", sc.s.pl[0].graveN)
	}
	for i := 0; i < 2 && !sc.onBF(giant); i++ {
		sc.passBoth(t) // (Teval's cast trigger resolves first)
	}
	if !sc.onBF(giant) {
		t.Fatal("the Giant did not resolve")
	}
}

// TestDelveGrantCredit: without the grant the same two lands do not afford
// the Giant, and no delve ask exists.
func TestDelveGrantCredit(t *testing.T) {
	sc, giant, _ := delveScen(t, false)
	if casts := sc.castOptions(giant); len(casts) != 0 {
		t.Fatalf("without Teval the Giant is offered %v on two lands", casts)
	}
}
