package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dredgeBodyScen is FDN's Plains table plus Golgari Thug (Dredge 4), a "draw
// two instead" replacement enchantment and a {0} "draw two" sorcery; both
// libraries hold 20 Plains, seat 0 is active.
func dredgeBodyScen(t *testing.T) (sc *scen, thug, doubler, drawTwo CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	thug = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Golgari Thug", Types: kop.TCreature, Cost: cost(1, kop.B), Power: 1, Toughness: 1, Dredge: 4})
	doubler = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Doubler", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
	drawTwo = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw Two", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)}}}}})
	sc = newScenTableLib(t, tbl, CardID(0), Config{NoElide: true})
	if !dredgeOn(sc.k) {
		t.Fatal("the table's dredge use is not set")
	}
	return sc, thug, doubler, drawTwo
}

// TestDredgeDrawMoreBodyAsks: a draw a ReplDrawMore replacement expands asks
// the dredge ask before each body card, as gorge drawFor does (the turn draw
// and an OpDraw card here; a body batch with no candidate draws plainly).
// Nothing stalls (StallDredge).
func TestDredgeDrawMoreBodyAsks(t *testing.T) {
	t.Run("the turn draw's body batch asks", func(t *testing.T) {
		sc, thug, doubler, _ := dredgeBodyScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		o := sc.grave(thug, 0, 1)[0]
		sc.perm(doubler, 0)
		lib := pl.libLen()
		toDrawStep(sc)
		dredgeAsk(t, sc, 2)                    // the turn draw's ask
		s.Apply(MakeAction(AkChoose, 0, 0, 1)) // the ordinary draw, replaced by two
		if s.g.stalled != 0 {
			t.Fatalf("stalled %d: the first body card asked instead", s.g.stalled)
		}
		dredgeAsk(t, sc, 2) // body card 1
		s.Apply(MakeAction(AkChoose, 0, 0, 1))
		dredgeAsk(t, sc, 2)                    // body card 2
		s.Apply(MakeAction(AkChoose, 0, 0, 0)) // dredge Thug
		if s.objs[o].zone != kop.ZoneHand || pl.handN != 2 || pl.libLen() != lib-5 || pl.graveN != 4 || pl.drawnTurn != 1 || s.g.stalled != 0 {
			t.Fatalf("Thug zone %d, hand %d, library %d (was %d), graveyard %d, drawn %d, stalled %d: want one drawn, then Thug dredged",
				s.objs[o].zone, pl.handN, pl.libLen(), lib, pl.graveN, pl.drawnTurn, s.g.stalled)
		}
		if d := s.Step(); d.Kind != DkPriority || s.step != kop.StepDraw || s.g.drawFirst {
			t.Fatalf("decision kind %d in step %d (drawFirst %v): want the draw step's priority", d.Kind, s.step, s.g.drawFirst)
		}
	})
	t.Run("an OpDraw's body batch asks", func(t *testing.T) {
		sc, thug, doubler, drawTwo := dredgeBodyScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		o := sc.grave(thug, 0, 1)[0]
		sc.perm(doubler, 0)
		spell := sc.hand(drawTwo, 0)
		lib := pl.libLen()
		sc.castCard(t, spell)
		sc.passBoth(t)
		dredgeAsk(t, sc, 2)                    // the first card's ask
		s.Apply(MakeAction(AkChoose, 0, 0, 1)) // the ordinary draw, replaced by two
		if s.g.stalled != 0 {
			t.Fatalf("stalled %d: the first body card asked instead", s.g.stalled)
		}
		dredgeAsk(t, sc, 2) // body card 1
		s.Apply(MakeAction(AkChoose, 0, 0, 1))
		dredgeAsk(t, sc, 2)                    // body card 2
		s.Apply(MakeAction(AkChoose, 0, 0, 0)) // dredge Thug
		// Thug is in hand: card two's own body batch has no candidate and
		// draws its two cards plainly (the spell joins the graveyard)
		if s.objs[o].zone != kop.ZoneHand || pl.handN != 4 || pl.libLen() != lib-7 || pl.graveN != 5 || pl.drawnTurn != 3 || s.g.stalled != 0 {
			t.Fatalf("Thug zone %d, hand %d, library %d (was %d), graveyard %d, drawn %d, stalled %d: want body one drawn, body two dredged, card two's two cards drawn",
				s.objs[o].zone, pl.handN, pl.libLen(), lib, pl.graveN, pl.drawnTurn, s.g.stalled)
		}
		if d := s.Step(); d.Kind != DkPriority || s.sp != 0 {
			t.Fatalf("decision kind %d, stack %d: want the spell resolved", d.Kind, s.sp)
		}
	})
	t.Run("a body batch with no candidate draws plainly", func(t *testing.T) {
		sc, _, doubler, _ := dredgeBodyScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.perm(doubler, 0)
		lib := pl.libLen()
		toDrawStep(sc)
		if d := s.Step(); d.Kind != DkPriority || pl.handN != 2 || pl.libLen() != lib-2 || pl.drawnTurn != 2 || s.g.stalled != 0 {
			t.Fatalf("decision kind %d, hand %d, library %d (was %d), drawn %d, stalled %d: want two drawn, no ask, no stall",
				d.Kind, pl.handN, pl.libLen(), lib, pl.drawnTurn, s.g.stalled)
		}
	})
}
