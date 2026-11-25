package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplDrawProgramDredgeBodyStalls: a ReplDrawProgram body whose Draw meets
// a dredge candidate cannot be suspended (gorge asks the dredge choice before
// each body card). The kernel declares the gap (StallDredge) instead of
// wiping the ask and silently skipping the body.
func TestReplDrawProgramDredgeBodyStalls(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	thug := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Golgari Thug", Types: kop.TCreature, Cost: cost(1, kop.B), Power: 1, Toughness: 1, Dredge: 4})
	host := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw Program", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram}, Prog: []kop.Instr{
			{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(2)},
			{Op: kop.OpLoseLife, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}}}}})
	sc := newScenTableLib(t, tbl, CardID(0), Config{NoElide: true})
	if !dredgeOn(sc.k) || sc.k.gen == nil || !sc.k.gen.anyDrawProgram {
		t.Fatal("precondition: the table lacks dredge use or a draw program")
	}
	s, pl := sc.s, &sc.s.pl[0]
	sc.grave(thug, 0, 1)
	if !sc.onBF(sc.perm(host, 0)) {
		t.Fatal("precondition: the replacement host is not on the battlefield")
	}
	toDrawStep(sc)
	dredgeAsk(t, sc, 2)
	s.Apply(MakeAction(AkChoose, 0, 0, 1)) // decline the turn draw's dredge
	if s.g.stalled != StallDredge {
		t.Fatalf("stalled = %d (res.ask %d, hand %d, life %d), want StallDredge %d: the body's dredge ask was dropped silently",
			s.g.stalled, s.res.ask, pl.handN, pl.life, StallDredge)
	}
}
