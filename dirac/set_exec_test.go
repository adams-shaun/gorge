package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExecSetRunsEachMember: a RefSet subject runs its op once per member of
// the set (a player op over every player, a destroy over the objects a filter
// keeps), restores the binding slot, and an op that takes no set panics with
// its name.
func TestExecSetRunsEachMember(t *testing.T) {
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
	fCre := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	tbl.Sets = []kop.SetExpr{
		{{Kind: kop.SetEachPlayer}},
		{{Kind: kop.SetSelf}, {Kind: kop.SetTriggerObj}, {Kind: kop.SetUnion}, {Kind: kop.SetAndFilter, Arg: uint16(fCre)}},
	}
	cd := tbl.Cards[lions]
	ab := len(cd.Abilities)
	cd.Abilities = append(cd.Abilities, kop.Ability{
		Prog: []kop.Instr{
			{Op: kop.OpGainLife, Obj: kop.RefSet, A: kop.Const(3)},
			{Op: kop.OpDestroy, Obj: kop.RefSet},
			{Op: kop.OpMill, Obj: kop.RefSet, A: kop.Const(1)},
		},
		SetRefs: []kop.SetRef{{PC: 0, Set: 0}, {PC: 1, Set: 1}, {PC: 2, Set: 0}},
	})
	tbl.Cards[lions] = cd
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	if !sc.k.gen.anyUnion {
		t.Fatal("precondition: a table with a RefSet subject takes the union arm")
	}
	a, b := sc.perm(lions, 0), sc.perm(lions, 1)
	e := stackEntry{kind: seTrigger, obj: a, srcVer: s.objs[a].ver, card: lions, ab: uint8(ab), ctrl: 0, trigObj: b}
	for i := range e.tgt {
		e.tgt[i] = NoTarget
	}
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	prog := tbl.Cards[lions].Abilities[ab].Prog

	l0, l1 := s.pl[0].life, s.pl[1].life
	s.res.pc = 0
	if !s.exec(sc.k, &prog[0]) {
		t.Fatal("the life gain suspended")
	}
	if s.pl[0].life != l0+3 || s.pl[1].life != l1+3 {
		t.Errorf("life %d/%d, want %d/%d: each player gains 3", s.pl[0].life, s.pl[1].life, l0+3, l1+3)
	}
	if !sc.onBF(a) || !sc.onBF(b) {
		t.Fatal("precondition: both Lions on the battlefield")
	}
	s.res.pc = 1
	s.exec(sc.k, &prog[1])
	if sc.onBF(a) || sc.onBF(b) {
		t.Errorf("a destroy over Self ∪ TriggeredCard left a Lions (self %v, trigger %v on the battlefield)", sc.onBF(a), sc.onBF(b))
	}
	if s.res.e.tgt[setBindSlot] != NoTarget || s.g.inLoop {
		t.Errorf("the binding slot holds %d after the walk (inLoop %v)", s.res.e.tgt[setBindSlot], s.g.inLoop)
	}
	s.res.pc = 2
	setPanics(t, "Mill over a set", "Mill", func() { s.exec(sc.k, &prog[2]) })
}
