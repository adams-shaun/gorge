package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapOrUntap: OpTapOrUntap asks the controller tap or untap (gorge's
// order: the state-changing choice first) and applies the answer.
func TestTapOrUntap(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	pl := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tou := kop.Instr{Op: kop.OpTapOrUntap, Obj: kop.RefTriggerObj}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Twiddle", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{tou}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	land := sc.perm(pl, 1)
	s := sc.s
	for _, tapped := range []bool{false, true} {
		if tapped {
			s.perms[sc.slot(land)].flags |= pfTapped
		}
		s.res.e.ctrl, s.res.e.card, s.res.e.ab, s.res.pc = 0, src, 0, 0
		s.res.e.trigObj = land
		if s.exec(sc.k, &tou) || s.res.ask != raGenTapOrUntap {
			t.Fatalf("tapped=%v: ask %d, want the tap-or-untap ask", tapped, s.res.ask)
		}
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		if d.Sub != ChGenTapOrUntap || d.N != 2 || d.Options[0].Source() != TCode(uint8(land)) {
			t.Fatalf("tapped=%v: sub %d with %d options", tapped, d.Sub, d.N)
		}
		if want := uint8(1); tapped == (d.Options[0].Mode() == want) {
			t.Fatalf("tapped=%v: first option mode %d, want the state-changing one", tapped, d.Options[0].Mode())
		}
		s.applyAskCold2(sc.k, d.Options[0]) // the state-changing answer
		if sc.tapped(land) == !tapped {
			continue
		}
		t.Fatalf("tapped=%v: the answer did not apply", tapped)
	}
	s.leave(sc.k, sc.slot(land), kop.ZoneGraveyard, false)
	s.res.pc = 0
	if !s.exec(sc.k, &tou) || s.res.ask != raNone {
		t.Fatal("a gone permanent: asked")
	}
}
