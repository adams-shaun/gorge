package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedUpkeep: an OpDelayedProgram with kop.DelayYourUpkeep waits
// through the end step (which fires only the end-step program) and the
// opponent's upkeep, and fires at its controller's next upkeep; one with
// kop.DelayUpkeep fires at the next upkeep of either player.
func TestDelayedUpkeep(t *testing.T) {
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
			{Op: kop.OpDelayedProgram, C: 1},
			{Op: kop.OpDelayedProgram, C: 1, A: kop.Const(kop.DelayYourUpkeep)},
			{Op: kop.OpDelayedProgram, C: 1, A: kop.Const(kop.DelayUpkeep)}}},
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
	if sc.s.g.ndp != 3 {
		t.Fatalf("%d delayed programs registered, want 3", sc.s.g.ndp)
	}
	sc.s.fireDelayedGen(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 2 {
		t.Fatalf("end step: %d pending, %d still registered, want 1 and 2", sc.s.npend, sc.s.g.ndp)
	}
	sc.s.npend = 0
	sc.s.active = 1
	sc.s.fireDelayedUpkeep(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 1 {
		t.Fatalf("opponent's upkeep: %d pending, %d still registered, want 1 (any player's) and 1", sc.s.npend, sc.s.g.ndp)
	}
	sc.s.npend = 0
	sc.s.active = 0
	sc.s.fireDelayedUpkeep(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("own upkeep: %d pending, %d still registered, want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
}

// TestDelayedEndCombat: an OpDelayedProgram with kop.DelayEndCombat waits
// through the upkeep and the end step and fires at the next end of combat
// step; the end-step program is not fired there.
func TestDelayedEndCombat(t *testing.T) {
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
			{Op: kop.OpDelayedProgram, C: 1, A: kop.Const(kop.DelayEndCombat)},
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
	sc.s.fireDelayedUpkeep(sc.k)
	if sc.s.npend != 0 || sc.s.g.ndp != 2 {
		t.Fatalf("upkeep: %d pending, %d registered, want 0 and 2", sc.s.npend, sc.s.g.ndp)
	}
	sc.s.fireDelayedEndCombat(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 1 {
		t.Fatalf("end of combat: %d pending, %d registered, want 1 and 1", sc.s.npend, sc.s.g.ndp)
	}
	sc.s.npend = 0
	sc.s.fireDelayedGen(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("end step: %d pending, %d registered, want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
}
