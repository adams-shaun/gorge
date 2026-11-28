package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExecSetLoseLifeOneLossGroup: a LoseLife over a RefSet of both players
// is one life-loss group (gorge effLoseLife's Begin/EndLifeLossBatch), so a
// LifeLostAll line ("one or more players each lose exactly 2 life") matches
// once as the walk ends, against the group's sums, not once per member.
func TestExecSetLoseLifeOneLossGroup(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}}
	if i, ok := set.Lookup("Savannah Lions"); !ok || !set.Supported()[i] {
		t.Skip("Savannah Lions not in the FDN table")
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var lions CardID
	for _, c := range da {
		if tbl.Cards[c].Name == "Savannah Lions" {
			lions = c
		}
	}
	tbl.Sets = []kop.SetExpr{{{Kind: kop.SetEachPlayer}}}
	cd := tbl.Cards[lions]
	ab := len(cd.Abilities)
	cd.Abilities = append(cd.Abilities,
		kop.Ability{
			Prog:    []kop.Instr{{Op: kop.OpLoseLife, Obj: kop.RefSet, A: kop.Const(2)}},
			SetRefs: []kop.SetRef{{PC: 0, Set: 0}},
		},
		kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigLifeLostAll, Whose: kop.WhoseAny, N: 2}},
	)
	tbl.Cards[lions] = cd
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	if !sc.k.gen.uses.has(useLifeLostAll) || s.lossB() == nil {
		t.Fatal("precondition: the table carries a LifeLostAll line and its group record")
	}
	a := sc.perm(lions, 0)
	if !sc.onBF(a) {
		t.Fatal("precondition: the listening Lions is on the battlefield")
	}
	e := stackEntry{kind: seTrigger, obj: a, srcVer: s.objs[a].ver, card: lions, ab: uint8(ab), ctrl: 0, trigObj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = NoTarget
	}
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	prog := tbl.Cards[lions].Abilities[ab].Prog

	l0, l1 := s.pl[0].life, s.pl[1].life
	np := s.npend
	s.res.pc = 0
	if !s.exec(sc.k, &prog[0]) {
		t.Fatal("the life loss suspended")
	}
	if s.pl[0].life != l0-2 || s.pl[1].life != l1-2 {
		t.Fatalf("precondition: life %d/%d, want %d/%d (each player loses 2)", s.pl[0].life, s.pl[1].life, l0-2, l1-2)
	}
	var got []pendingTrig
	for _, p := range s.pend[np:s.npend] {
		if p.card == lions && int(p.ab) == ab+1 {
			got = append(got, p)
		}
	}
	if len(got) != 1 {
		t.Fatalf("LifeLostAll queued %d times, want once for the whole LoseLife", len(got))
	}
	if got[0].trigAmt != 2 {
		t.Errorf("LifeLostAll amount %d, want the group's summed 2", got[0].trigAmt)
	}
	if b := s.lossB(); b.on {
		t.Error("the life-loss group is still open after the walk")
	}
}
