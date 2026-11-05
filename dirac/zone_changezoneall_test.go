package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newWeftScen is a matchup of Weftwalking decks over the weftwalking pool,
// libraries of Plains, empty hands; it returns Weftwalking's lowered sweep.
func newWeftScen(t *testing.T) (*scen, CardID, kop.Instr) {
	t.Helper()
	set := cardsettest.Set(t, "weftwalking")
	deck := []cardset.Entry{{Name: "Weftwalking", Count: 4}, {Name: "Plains", Count: 36}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc := &scen{s: s, k: s.k()}
	for _, a := range tbl.Cards[id["Weftwalking"]].Abilities {
		for _, in := range a.Prog {
			if in.Op == kop.OpChangeZoneAllEx {
				return sc, id["Plains"], in
			}
		}
	}
	t.Fatal("Weftwalking has no OpChangeZoneAllEx")
	return nil, 0, kop.Instr{}
}

// libTopOrder is seat p's library from the top down, n cards.
func libTopOrder(s *State, p uint8, n int) []Obj {
	pl := &s.pl[p]
	return slices.Clone(pl.lib[pl.libLo : int(pl.libLo)+n])
}

// TestChangeZoneAllGraveHandOrder: a Graveyard,Hand sweep into the library
// places the graveyard cards above the hand cards (gorge moves zone-major
// and places all moved cards in move order), not the second zone on top.
func TestChangeZoneAllGraveHandOrder(t *testing.T) {
	sc, plains, in := newWeftScen(t)
	s := sc.s
	g := sc.grave(plains, 0, 2)
	h := []Obj{sc.hand(plains, 0), sc.hand(plains, 0)}
	pl := &s.pl[0]
	if pl.graveN != 2 || pl.handN != 2 {
		t.Fatalf("setup: grave %d hand %d", pl.graveN, pl.handN)
	}
	before := int(pl.libHi - pl.libLo)
	s.res.e = stackEntry{kind: seSpell, ctrl: 0}
	if in.B != kop.Const(int(kop.ZoneHand)) || in.C>>3&kop.MvGraveExile == 0 {
		t.Fatalf("lowered sweep %+v is not the Graveyard,Hand form", in)
	}
	s.execChangeZoneAll(sc.k, &in, false)
	if got := int(pl.libHi-pl.libLo) - before; got != 4 {
		t.Fatalf("library grew by %d, want 4", got)
	}
	want := []Obj{g[0], g[1], h[0], h[1]}
	if got := libTopOrder(s, 0, 4); !slices.Equal(got, want) {
		t.Fatalf("library top %v, want graveyard then hand %v", got, want)
	}
	if pl.graveN != 0 || pl.handN != 0 {
		t.Errorf("zones not emptied: grave %d hand %d", pl.graveN, pl.handN)
	}
}

// TestChangeZoneAllGraveExileLegacy: B.N == 0 keeps sweeping Graveyard then
// Exile.
func TestChangeZoneAllGraveExileLegacy(t *testing.T) {
	sc, plains, in := newWeftScen(t)
	s := sc.s
	g := sc.grave(plains, 0, 1)[0]
	e := sc.obj(plains, 0)
	s.place(e, kop.ZoneExile, false)
	h := sc.hand(plains, 0)
	in.B = kop.ValueRef{}
	in.C = int16(kop.ZoneHand) | kop.MvGraveExile<<3
	s.res.e = stackEntry{kind: seSpell, ctrl: 0}
	s.execChangeZoneAll(sc.k, &in, false)
	for _, o := range []Obj{g, e, h} {
		if s.objs[o].zone != kop.ZoneHand {
			t.Errorf("object %d in zone %v, want Hand", o, s.objs[o].zone)
		}
	}
	if s.pl[0].graveN != 0 || s.pl[0].exileN != 0 {
		t.Errorf("grave %d exile %d, want both swept", s.pl[0].graveN, s.pl[0].exileN)
	}
}
