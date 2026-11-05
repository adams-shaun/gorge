package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedYourEndStep: an OpDelayedProgram with kop.DelayYourEndStep waits
// through another player's end step (which fires the any-player end-step
// program) and through the turn change, and fires at its controller's next end
// step. ValidPlayer$ You is a player gate, not a different step.
func TestDelayedYourEndStep(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bauble", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Prog: []kop.Instr{
			{Op: kop.OpDelayedProgram, C: 1, A: kop.Const(kop.DelayYourEndStep)},
			{Op: kop.OpDelayedProgram, C: 1}}},
		{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Prog: gain}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyDelayUp {
		t.Fatal("genPrep.anyDelayUp not set")
	}
	o := sc.perm(c, 0)
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card = 0, o, c
	for i := range tbl.Cards[c].Abilities[0].Prog {
		sc.s.exec(sc.k, &tbl.Cards[c].Abilities[0].Prog[i])
	}
	if sc.s.g.ndp != 2 {
		t.Fatalf("%d delayed programs registered, want 2", sc.s.g.ndp)
	}
	sc.s.active = 1
	sc.s.fireDelayedGen(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 1 {
		t.Fatalf("opponent's end step: %d pending, %d still registered, want 1 (any player's) and 1 (controller's next)", sc.s.npend, sc.s.g.ndp)
	}
	sc.s.dropDelayedBeginCombat()
	if sc.s.g.ndp != 1 {
		t.Fatalf("turn change dropped the controller's-end-step program: %d registered", sc.s.g.ndp)
	}
	sc.s.npend = 0
	sc.s.active = 0
	sc.s.fireDelayedGen(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("controller's end step: %d pending, %d still registered, want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
}
