package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dredgePathsScen is FDN's Plains table plus Golgari Thug (Dredge 4), a
// connive sorcery, a Recruit sorcery, a draw-up-to sorcery and the Recruit
// token card; both libraries hold 20 Plains, seat 0 is active.
func dredgePathsScen(t *testing.T) (sc *scen, thug, conniveSpell, recruitSpell, uptoSpell, plainsID CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plainsID = CardID(0)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plainsID = CardID(i)
		}
	}
	thug = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Golgari Thug", Types: kop.TCreature, Cost: cost(1, kop.B), Power: 1, Toughness: 1, Dredge: 4})
	conniveSpell = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Connive", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpConnive, Obj: kop.RefSelf, A: kop.Const(1)}}}}})
	recruitSpell = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Recruit", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtRecruit))}}}}})
	uptoSpell = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw Upto", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtDrawUpto)), Obj: kop.RefController, B: kop.Const(1)}}}}})
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "w_1_1_human_soldier", Types: kop.TCreature, Power: 1, Toughness: 1})
	sc = newScenTableLib(t, tbl, plainsID, Config{NoElide: true})
	if !dredgeOn(sc.k) {
		t.Fatal("the table's dredge use is not set")
	}
	return sc, thug, conniveSpell, recruitSpell, uptoSpell, plainsID
}

// TestDredgeOtherDrawPaths: gorge drawFor asks before every effect draw, so
// the kernel poses the dredge ask on the draw paths past the turn draw and
// OpDraw: connive, Recruit, the monarch's end-step draw and a Draw Upto's
// body (each a DkChoose ChGenBranch of the candidates, then the draw), while
// a draw paid as a resolution cost draws plainly (gorge rules' draw-bearing
// triggered cost does not ask). Nothing stalls (StallDredge).
func TestDredgeOtherDrawPaths(t *testing.T) {
	t.Run("connive asks, then discards", func(t *testing.T) {
		sc, thug, cv, _, _, plainsID := dredgePathsScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.grave(thug, 0, 1)
		spell := sc.hand(cv, 0)
		sc.hand(plainsID, 0) // the hand holds more than the draw: the discard is asked
		lib, hand := pl.libLen(), pl.handN
		sc.castCard(t, spell)
		sc.passBoth(t)
		dredgeAsk(t, sc, 2)
		s.Apply(MakeAction(AkChoose, 0, 0, 1)) // the ordinary draw
		if pl.handN != hand || pl.libLen() != lib-1 || s.g.stalled != 0 {
			t.Fatalf("hand %d (was %d), library %d (was %d), stalled %d: want one drawn, no stall",
				pl.handN, hand, pl.libLen(), lib, s.g.stalled)
		}
		// the draw done, connive's discard is asked (the hand holds more
		// than the one card drawn)
		d := s.Step()
		if d.Kind != DkChoose || d.N < 1 {
			t.Fatalf("decision kind %d with %d options, want the connive discard", d.Kind, d.N)
		}
		s.Apply(d.Options[0])
		if pl.handN != hand-1 || s.sp != 0 {
			t.Fatalf("hand %d (was %d), stack %d: want one discarded and the spell resolved", pl.handN, hand-1, s.sp)
		}
		if d := s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
	})
	t.Run("recruit asks and dredges", func(t *testing.T) {
		sc, thug, _, rc, _, plainsID := dredgePathsScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		o := sc.grave(thug, 0, 1)[0]
		spell := sc.hand(rc, 0)
		sc.hand(plainsID, 0) // the hand holds more than one card: the discard is asked
		lib, hand := pl.libLen(), pl.handN
		sc.castCard(t, spell)
		sc.passBoth(t)
		dredgeAsk(t, sc, 2)
		s.Apply(MakeAction(AkChoose, 0, 0, 0)) // dredge Thug: four milled, nothing drawn
		if s.objs[o].zone != kop.ZoneHand || pl.handN != hand || pl.libLen() != lib-4 || pl.drawnTurn != 0 || s.g.stalled != 0 {
			t.Fatalf("Thug zone %d, hand %d (was %d), library %d (was %d), drawn %d, stalled %d: want Thug returned, four milled, nothing drawn",
				s.objs[o].zone, pl.handN, hand, pl.libLen(), lib, pl.drawnTurn, s.g.stalled)
		}
		// the draw done, Recruit's discard is asked; the pick is a land, so
		// no token
		d := s.Step()
		if d.Kind != DkChoose || d.Sub != ChRecruitDiscard || d.N < 1 {
			t.Fatalf("decision kind %d sub %d with %d options, want the Recruit discard", d.Kind, d.Sub, d.N)
		}
		s.Apply(d.Options[0])
		if pl.handN != hand-1 || s.sp != 0 {
			t.Fatalf("hand %d (was %d), stack %d: want one discarded and the spell resolved", pl.handN, hand-1, s.sp)
		}
		if d := s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
	})
	t.Run("a resolution draw cost draws plainly", func(t *testing.T) {
		sc, thug, _, _, _, _ := dredgePathsScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.grave(thug, 0, 1)
		h, l := pl.handN, pl.libLen()
		s.payDraw(0, 2)
		if pl.handN != h+2 || pl.libLen() != l-2 || s.g.stalled != 0 || s.res.ask != raNone {
			t.Fatalf("hand %d (was %d), library %d (was %d), stalled %d, ask %d: want two drawn, no ask, no stall",
				pl.handN, h, pl.libLen(), l, s.g.stalled, s.res.ask)
		}
	})
	t.Run("the monarch's end-step draw asks", func(t *testing.T) {
		sc, thug, _, _, _, _ := dredgePathsScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.grave(thug, 0, 1)
		s.becomeMonarch(sc.k, 0)
		h, l := pl.handN, pl.libLen()
		s.push(stackEntry{kind: seTrigger, obj: NoObj, ab: abDelayed, trigAmt: monDrawAmt})
		s.resolveTop(sc.k)
		if s.res.ask != raDredgeRes {
			t.Fatalf("ask %d, want the monarch draw's dredge ask", s.res.ask)
		}
		d := s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenBranch || d.Player != 0 || d.N != 2 {
			t.Fatalf("decision kind %d sub %d player %d with %d options, want seat 0's dredge ask", d.Kind, d.Sub, d.Player, d.N)
		}
		s.Apply(MakeAction(AkChoose, 0, 0, 1)) // the ordinary draw
		if pl.handN != h+1 || pl.libLen() != l-1 || s.g.stalled != 0 || s.sp != 0 {
			t.Fatalf("hand %d (was %d), library %d (was %d), stalled %d, stack %d: want one drawn, the trigger done",
				pl.handN, h, pl.libLen(), l, s.g.stalled, s.sp)
		}
	})
	t.Run("draw up to asks, then its body draw asks", func(t *testing.T) {
		sc, thug, _, _, up, _ := dredgePathsScen(t)
		s, pl := sc.s, &sc.s.pl[0]
		sc.grave(thug, 0, 1)
		spell := sc.hand(up, 0)
		lib, hand := pl.libLen(), pl.handN
		sc.castCard(t, spell)
		sc.passBoth(t)
		d := s.Step() // the draw-up-to pick
		if d.Kind != DkChoose || d.Sub != ChDrawUpto || d.N < 1 {
			t.Fatalf("decision kind %d sub %d with %d options, want the draw-up-to pick", d.Kind, d.Sub, d.N)
		}
		s.Apply(d.Options[d.N-1]) // the count's one card (the Done option ends it first)
		dredgeAsk(t, sc, 2)       // the body draw asks
		s.Apply(MakeAction(AkChoose, 0, 0, 1))
		if pl.handN != hand || pl.libLen() != lib-1 || s.g.stalled != 0 {
			t.Fatalf("hand %d (was %d), library %d (was %d), stalled %d: want one drawn, no stall",
				pl.handN, hand, pl.libLen(), lib, s.g.stalled)
		}
		if d := s.Step(); d.Kind != DkPriority || s.sp != 0 {
			t.Fatalf("decision kind %d, stack %d: want the spell resolved", d.Kind, s.sp)
		}
	})
}
