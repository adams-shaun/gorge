package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSameNameTokenPrintedName: two card entries that print one name (two
// token scripts with the same Name: line, whose table Names are their
// distinct stems; CardDef.FaceName) are one name to a name predicate, as
// gorge compares e.Name strings. A card with another printed name is not.
func TestSameNameTokenPrintedName(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	var deck []cardset.Entry
	for _, n := range []string{"Ambush Wolf", "Axgard Cavalry", "Aegis Turtle"} {
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
	a, b, other := id["Ambush Wolf"], id["Axgard Cavalry"], id["Aegis Turtle"]
	tbl.Cards[a].FaceName = "Soldier Token"
	tbl.Cards[b].FaceName = "Soldier Token"
	tbl.Filters = append(tbl.Filters[:len(tbl.Filters):len(tbl.Filters)],
		kop.Filter{Flags: kop.FToken, Ctrl: kop.CtrlOpp, Pred: kop.PredSameNameT0},
		kop.Filter{Flags2: kop.F2TargetedBy},
		kop.Filter{Pred: kop.PredNamed, PredN: int16(b)})
	fb := kop.FilterID(len(tbl.Filters) - 3)
	tbl.Filters[fb+1].Or = fb
	named := fb + 2
	if tbl.CardName(a) != tbl.CardName(b) || tbl.Cards[a].Name == tbl.Cards[b].Name || tbl.CardName(a) == tbl.CardName(other) {
		t.Fatal("setup: the printed names are not as built")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k

	tok := func(c CardID, p uint8) Obj {
		o := s.newToken(c, p)
		s.enter(k, o, p, false, kop.ZoneNone)
		s.npend = 0
		return o
	}
	x := sc.perm(a, 1)
	sameName := tok(b, 1)
	diff := tok(other, 1)
	for _, o := range []Obj{x, sameName, diff} {
		if !sc.onBF(o) {
			t.Fatalf("setup: object %d is not on the battlefield", o)
		}
	}
	if s.cardAt(s.objs[x].slot) == s.cardAt(s.objs[sameName].slot) {
		t.Fatal("setup: the permanents share a CardID")
	}
	if s.nameCardAt(s.objs[x].slot) != s.nameCardAt(s.objs[sameName].slot) {
		t.Error("nameCardAt: permanents printing one name read as two names")
	}
	if s.nameCardAt(s.objs[x].slot) == s.nameCardAt(s.objs[diff].slot) {
		t.Error("nameCardAt: permanents printing two names read as one name")
	}
	// PredNamed with the other card's id: the printed name matches
	if !s.predPermGen(k, &tbl.Filters[named], s.objs[x].slot, NoSlot, layerFull) {
		t.Error("PredNamed: a permanent printing the named card's name does not match")
	}
	if s.predPermGen(k, &tbl.Filters[named], s.objs[diff].slot, NoSlot, layerFull) {
		t.Error("PredNamed: a permanent printing another name matches")
	}
	// SameName$ over the target: the same-printed-name token is swept too
	sc.resolveAt(x, s.objs[x].ver)
	in := kop.Instr{Op: kop.OpChangeZoneAllEx, C: int16(kop.ZoneExile), F: fb + 1}
	s.execChangeZoneAll(k, &in, false)
	if s.objs[sameName].zone == kop.ZoneBattlefield {
		t.Error("SameName sweep: the same-printed-name token stayed on the battlefield")
	}
	if !sc.onBF(diff) {
		t.Error("SameName sweep: the other-named token left the battlefield")
	}
}
