package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExplore: OpExplore (gorge exploreOnce) puts a revealed land into the
// hand without asking; a nonland card poses the graveyard-or-top ask, and
// the answer puts a +1/+1 counter on the explorer, then the card into the
// graveyard (option 0) or leaves it on top.
func TestExplore(t *testing.T) {
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lc := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Savannah Lions" {
			lc = CardID(i)
		}
	}
	ex := kop.Instr{Op: kop.OpExplore, Obj: kop.RefSelf}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Explorer", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{ex}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	lions := sc.perm(lc, 0)
	pl := &s.pl[0]
	land := pl.lib[pl.libLo]
	s.res.e.ctrl, s.res.e.obj, s.res.e.card, s.res.e.ab, s.res.pc = 0, lions, src, 0, 0
	s.res.e.srcVer = s.objs[lions].ver
	hand := pl.handN
	if !s.exec(sc.k, &ex) || s.objs[land].zone != kop.ZoneHand || pl.handN != hand+1 {
		t.Fatalf("a land on top: zone %d, want it in hand unasked", s.objs[land].zone)
	}
	for _, grave := range []bool{true, false} {
		nl := sc.obj(lc, 0) // a nonland card on top
		s.objs[nl].zone = kop.ZoneLibrary
		pl.libTop(nl)
		if s.exec(sc.k, &ex) || s.res.ask != raGenExplore {
			t.Fatalf("a nonland card on top: ask %d, want the explore ask", s.res.ask)
		}
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		if d.Sub != ChGenExplore || d.N != 2 || d.Options[0].Source() != TCode(uint8(nl)) || d.Options[0].Mode() != 0 {
			t.Fatalf("explore ask: sub %d with %d options", d.Sub, d.N)
		}
		before := s.perms[sc.slot(lions)].ctrLow(int(kop.CtrP1P1))
		a := d.Options[1]
		if grave {
			a = d.Options[0]
		}
		s.applyAskCold2(sc.k, a)
		if got := s.perms[sc.slot(lions)].ctrLow(int(kop.CtrP1P1)); got != before+1 {
			t.Fatalf("grave=%v: %d +1/+1 counters, want %d", grave, got, before+1)
		}
		want := kop.ZoneLibrary
		if grave {
			want = kop.ZoneGraveyard
		}
		if z := s.objs[nl].zone; z != want {
			t.Fatalf("grave=%v: the revealed card in zone %d, want %d", grave, z, want)
		}
		if !grave && pl.lib[pl.libLo] != nl {
			t.Fatal("kept: the card is not on top")
		}
		s.res.pc = 0
		if !grave {
			pl.libLo++ // drop it for the next round
			s.objs[nl].zone = kop.ZoneNone
		}
	}
}
