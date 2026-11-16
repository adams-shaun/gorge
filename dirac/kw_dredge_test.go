package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dredgeScen is FDN's Plains table plus two dredge creatures (Golgari Thug's
// Dredge 4, Stinkweed Imp's Dredge 5) and a {0} "draw two" sorcery; both
// libraries hold 20 Plains, every other zone is empty, seat 0 is active.
func dredgeScen(t *testing.T) (sc *scen, thug, imp, divine CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := CardID(0)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	thug = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Golgari Thug", Types: kop.TCreature, Cost: cost(1, kop.B), Power: 1, Toughness: 1, Dredge: 4})
	imp = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Stinkweed Imp", Types: kop.TCreature, Cost: cost(2, kop.B), Power: 1, Toughness: 2, Dredge: 5})
	divine = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw Two", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)}}}}})
	sc = newScenTableLib(t, tbl, plains, Config{NoElide: true})
	if !dredgeOn(sc.k) {
		t.Fatal("the table's dredge use is not set")
	}
	return sc, thug, imp, divine
}

// dredgeAsk steps to the pending decision and fails unless it is seat 0's
// dredge ask with n options (the candidates, then the draw).
func dredgeAsk(t *testing.T, sc *scen, n int) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenBranch || d.Player != 0 || int(d.N) != n {
		t.Fatalf("decision kind %d sub %d player %d with %d options, want seat 0's dredge ask with %d", d.Kind, d.Sub, d.Player, d.N, n)
	}
	for i := 0; i < n; i++ {
		if d.Options[i].Mode() != uint8(i) {
			t.Fatalf("option %d has mode %d, want %d (gorge's KModes index)", i, d.Options[i].Mode(), i)
		}
	}
}

// toDrawStep begins seat 0's draw step.
func toDrawStep(sc *scen) {
	sc.s.step, sc.s.ctl = kop.StepDraw, ctlStepBegin
}

// TestDredgeReplacesDraw: CR 702.52a as gorge drawFor poses it. The turn draw
// with Golgari Thug in the graveyard asks [dredge Thug, draw]; the dredge
// mills four and returns Thug (no card drawn), the draw takes the top card
// and leaves Thug. A three-card library asks nothing. Two dredge cards are
// both offered in graveyard order, the draw last. A draw-two asks before
// each card.
func TestDredgeReplacesDraw(t *testing.T) {
	t.Run("turn draw dredges", func(t *testing.T) {
		sc, thug, _, _ := dredgeScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		o := sc.grave(thug, 0, 1)[0]
		lib, drawn := pl.libLen(), pl.drawnTurn
		toDrawStep(sc)
		dredgeAsk(t, sc, 2)
		s.Apply(MakeAction(AkChoose, 0, 0, 0))
		if s.objs[o].zone != kop.ZoneHand || pl.handN != 1 || pl.libLen() != lib-4 || pl.graveN != 4 || pl.drawnTurn != drawn {
			t.Fatalf("Thug zone %d, hand %d, library %d (was %d), graveyard %d, drawn %d: want Thug alone in hand, four milled, nothing drawn",
				s.objs[o].zone, pl.handN, pl.libLen(), lib, pl.graveN, pl.drawnTurn)
		}
		if d := s.Step(); d.Kind != DkPriority || s.step != kop.StepDraw || s.g.drawFirst {
			t.Fatalf("decision kind %d in step %d (drawFirst %v): want the draw step's priority", d.Kind, s.step, s.g.drawFirst)
		}
	})
	t.Run("turn draw declined", func(t *testing.T) {
		sc, thug, _, _ := dredgeScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		o := sc.grave(thug, 0, 1)[0]
		top, lib := pl.lib[pl.libLo], pl.libLen()
		toDrawStep(sc)
		dredgeAsk(t, sc, 2)
		s.Apply(MakeAction(AkChoose, 0, 0, 1))
		if s.objs[o].zone != kop.ZoneGraveyard || s.objs[top].zone != kop.ZoneHand || pl.libLen() != lib-1 || pl.drawnTurn != 1 {
			t.Fatalf("Thug zone %d, top card zone %d, library %d (was %d), drawn %d: want the top card drawn, Thug left",
				s.objs[o].zone, s.objs[top].zone, pl.libLen(), lib, pl.drawnTurn)
		}
		if d := s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
	})
	t.Run("library too small", func(t *testing.T) {
		sc, thug, _, _ := dredgeScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.grave(thug, 0, 1)
		pl.libLo = pl.libHi - 3
		toDrawStep(sc)
		if d := s.Step(); d.Kind != DkPriority || pl.handN != 1 || pl.libLen() != 2 {
			t.Fatalf("decision kind %d, hand %d, library %d: want no ask, one card drawn", d.Kind, pl.handN, pl.libLen())
		}
	})
	t.Run("two candidates in graveyard order", func(t *testing.T) {
		sc, thug, imp, _ := dredgeScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		th := sc.grave(thug, 0, 1)[0]
		im := sc.grave(imp, 0, 1)[0]
		lib := pl.libLen()
		toDrawStep(sc)
		dredgeAsk(t, sc, 3)
		s.Apply(MakeAction(AkChoose, 0, 0, 1)) // the second card in the graveyard: the Imp
		if s.objs[im].zone != kop.ZoneHand || s.objs[th].zone != kop.ZoneGraveyard || pl.libLen() != lib-5 {
			t.Fatalf("Imp zone %d, Thug zone %d, library %d (was %d): want the Imp returned and five milled", s.objs[im].zone, s.objs[th].zone, pl.libLen(), lib)
		}
	})
	t.Run("draw two asks twice", func(t *testing.T) {
		sc, thug, _, divine := dredgeScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		o := sc.grave(thug, 0, 1)[0]
		spell := sc.hand(divine, 0)
		lib := pl.libLen()
		sc.castCard(t, spell)
		sc.passBoth(t)
		dredgeAsk(t, sc, 2)
		s.Apply(MakeAction(AkChoose, 0, 0, 0)) // the first card: dredge Thug
		// Thug is in hand: the second card has no candidate and is drawn unasked
		if s.objs[o].zone != kop.ZoneHand || pl.libLen() != lib-5 || pl.drawnTurn != 1 {
			t.Fatalf("Thug zone %d, library %d (was %d), drawn %d: want Thug returned, four milled, then one drawn", s.objs[o].zone, pl.libLen(), lib, pl.drawnTurn)
		}
		if d := s.Step(); d.Kind != DkPriority || s.sp != 0 {
			t.Fatalf("decision kind %d, stack %d: want the spell resolved", d.Kind, s.sp)
		}

		sc, thug, _, divine = dredgeScen(t)
		s, pl = sc.s, &sc.s.pl[0]
		o = sc.grave(thug, 0, 1)[0]
		spell = sc.hand(divine, 0)
		lib = pl.libLen()
		sc.castCard(t, spell)
		sc.passBoth(t)
		dredgeAsk(t, sc, 2)
		s.Apply(MakeAction(AkChoose, 0, 0, 1)) // the first card: drawn
		dredgeAsk(t, sc, 2)                    // the second card asks again
		s.Apply(MakeAction(AkChoose, 0, 0, 0))
		if s.objs[o].zone != kop.ZoneHand || pl.libLen() != lib-5 || pl.drawnTurn != 1 {
			t.Fatalf("Thug zone %d, library %d (was %d), drawn %d: want one drawn, then Thug dredged", s.objs[o].zone, pl.libLen(), lib, pl.drawnTurn)
		}
		if d := s.Step(); d.Kind != DkPriority || s.sp != 0 {
			t.Fatalf("decision kind %d, stack %d: want the spell resolved", d.Kind, s.sp)
		}
	})
	t.Run("an unasked draw path stalls", func(t *testing.T) {
		sc, thug, _, _ := dredgeScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.grave(thug, 0, 1)
		h := pl.handN
		s.draw(0) // a draw with no ask posed (connive, recruit, ...)
		if s.g.stalled != StallDredge || pl.handN != h {
			t.Fatalf("stalled %d, hand %d (was %d): want StallDredge and no card drawn", s.g.stalled, pl.handN, h)
		}
	})
}
