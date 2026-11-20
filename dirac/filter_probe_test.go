package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestProbeSetReadsTheOverlay: a filter and an amount named by overlay indexes
// read the game's own cards and board, and the probes leave the state alone.
func TestProbeSetReadsTheOverlay(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "blb")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = da[0]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	land := kop.Filter{Types: kop.TLand}
	you := kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou}
	fs := append(slices.Clone(tbl.Filters), land, you)
	nl := kop.FilterID(len(tbl.Filters))
	ps, err := s.NewProbeSet(fs, slices.Clone(tbl.Exprs))
	if err != nil {
		t.Fatal(err)
	}
	var o Obj = NoObj
	for i := Obj(0); i < s.nobj; i++ {
		if s.objs[i].zone == kop.ZoneLibrary && s.objs[i].owner() == 0 {
			o = i
			break
		}
	}
	if o == NoObj {
		t.Fatal("no library card of seat 0")
	}
	if !s.ProbeFilter(ps, nl, o, NoObj, 0) || !s.ProbeFilter(ps, nl+1, o, NoObj, 0) || s.ProbeFilter(ps, nl+1, o, NoObj, 1) {
		t.Errorf("Plains in the library: land %v, your land %v, seat 1's land %v; want true true false",
			s.ProbeFilter(ps, nl, o, NoObj, 0), s.ProbeFilter(ps, nl+1, o, NoObj, 0), s.ProbeFilter(ps, nl+1, o, NoObj, 1))
	}
	if z := s.ProbeObjZone(o); z != kop.ZoneLibrary {
		t.Errorf("ProbeObjZone %v", z)
	}
	if got := s.ProbeValue(ps, kop.Const(3), NoObj, 0); got != 3 {
		t.Errorf("ProbeValue const: %d", got)
	}
}
