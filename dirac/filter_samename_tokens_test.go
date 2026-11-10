package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSameNameT0TokensBattlefield: Legions to Ashes' sweep, built by hand the
// way kgen lowers it (TestChangeZoneAllSameNameLowers pins that lowering): a
// battlefield-to-exile ChangeZoneAll over the target itself (targetedBy) OR an
// opponent-controlled token reading PredSameNameT0. Seat 1 controls the target
// X, a token and a nontoken named X and a token with another name; seat 0
// controls a token named X. The target and seat 1's same-named token are
// exiled; the rest stay.
func TestSameNameT0TokensBattlefield(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	var deck []cardset.Entry
	for _, n := range []string{"Ambush Wolf", "Axgard Cavalry"} {
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
	tbl.Filters = append(slices.Clip(tbl.Filters),
		kop.Filter{Flags: kop.FToken, Ctrl: kop.CtrlOpp, Pred: kop.PredSameNameT0},
		kop.Filter{Flags2: kop.F2TargetedBy})
	fb := kop.FilterID(len(tbl.Filters) - 2)
	tbl.Filters[fb+1].Or = fb
	in := kop.Instr{Op: kop.OpChangeZoneAllEx, C: int16(kop.ZoneExile), F: fb + 1}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k

	tok := func(c CardID, p uint8) Obj {
		o := s.newToken(c, p)
		s.enter(k, o, p, false, kop.ZoneNone)
		s.npend = 0
		return o
	}
	x := sc.perm(id["Ambush Wolf"], 1)
	sameTok := tok(id["Ambush Wolf"], 1)
	sameNontok := sc.perm(id["Ambush Wolf"], 1)
	theirTok := tok(id["Ambush Wolf"], 0)
	otherTok := tok(id["Axgard Cavalry"], 1)
	if s.objs[sameTok].of&ofToken == 0 || s.objs[theirTok].of&ofToken == 0 || s.objs[sameNontok].of&ofToken != 0 || s.objs[x].of&ofToken != 0 {
		t.Fatal("setup: token flags are not as built")
	}
	for _, o := range []Obj{x, sameTok, sameNontok, theirTok, otherTok} {
		if !sc.onBF(o) {
			t.Fatalf("setup: object %d is not on the battlefield", o)
		}
	}
	sc.resolveAt(x, s.objs[x].ver)
	if s.res.bad&1 != 0 {
		t.Fatal("setup: the target is not legal")
	}
	s.execChangeZoneAll(k, &in, false)
	for name, o := range map[string]Obj{"the target": x, "seat 1's same-named token": sameTok} {
		if z := s.objs[o].zone; z == kop.ZoneBattlefield {
			t.Errorf("%s still on the battlefield", name)
		}
	}
	for name, o := range map[string]Obj{
		"seat 1's same-named nontoken": sameNontok,
		"seat 0's same-named token":    theirTok,
		"seat 1's other-named token":   otherTok,
	} {
		if !sc.onBF(o) {
			t.Errorf("%s left the battlefield (zone %v)", name, s.objs[o].zone)
		}
	}
}
