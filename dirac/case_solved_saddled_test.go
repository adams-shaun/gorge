package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCaseSolvedIsNotSaddled: a Case's solved designation (CR 719.3b) is
// its own bit, not the saddled designation (CR 702.171b). OpSaddle C=1
// marks a Case solved; it must not read IsSaddled. OpSaddle C=0 is the
// saddled designation; it must not read solved, and the turn-begin clear
// (unsaddleAll) must drop saddled but keep solved.
func TestCaseSolvedIsNotSaddled(t *testing.T) {
	set := cardsettest.Set(t, "case")
	deck := []cardset.Entry{
		{Name: "Case of the Gorgon's Kiss", Count: 1},
		{Name: "Ornithopter", Count: 1},
		{Name: "Island", Count: 8},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	s := sc.s

	sc.k.t.Filters = append(sc.k.t.Filters,
		kop.Filter{Pred: kop.PredSolved, PredN: 1},
		kop.Filter{Flags2: kop.F2Saddled})
	solved := kop.FilterID(len(sc.k.t.Filters) - 2)
	saddled := kop.FilterID(len(sc.k.t.Filters) - 1)

	caseObj := sc.perm(id["Case of the Gorgon's Kiss"], 0)
	other := sc.perm(id["Ornithopter"], 0)
	sl, osl := sc.slot(caseObj), sc.slot(other)
	if s.matchPerm(sc.k, solved, sl, 0, NoSlot) || s.matchPerm(sc.k, saddled, sl, 0, NoSlot) {
		t.Fatal("precondition: the Case starts neither solved nor saddled")
	}
	if s.matchPerm(sc.k, solved, osl, 0, NoSlot) || s.matchPerm(sc.k, saddled, osl, 0, NoSlot) {
		t.Fatal("precondition: the Ornithopter starts neither solved nor saddled")
	}

	// OpSaddle C=1 is a Case's solve, not a saddle.
	s.res.e.kind, s.res.e.obj = seAbility, caseObj
	s.res.e.srcVer = s.objs[caseObj].ver
	s.saddle(sc.k, &kop.Instr{Op: kop.OpSaddle, Obj: kop.RefSelf, C: 1})
	if !s.matchPerm(sc.k, solved, sl, 0, NoSlot) {
		t.Fatal("OpSaddle C=1 did not mark the Case solved")
	}
	if s.matchPerm(sc.k, saddled, sl, 0, NoSlot) {
		t.Fatal("a solved Case reads IsSaddled")
	}
	s.unsaddleAll(sc.k)
	if !s.matchPerm(sc.k, solved, sl, 0, NoSlot) {
		t.Fatal("the solve did not survive the turn-begin clear")
	}

	// OpSaddle C=0 is the saddled designation, not a solve.
	s.res.e.obj = other
	s.res.e.srcVer = s.objs[other].ver
	s.saddle(sc.k, &kop.Instr{Op: kop.OpSaddle, Obj: kop.RefSelf})
	if !s.matchPerm(sc.k, saddled, osl, 0, NoSlot) {
		t.Fatal("OpSaddle C=0 did not mark the permanent saddled")
	}
	if s.matchPerm(sc.k, solved, osl, 0, NoSlot) {
		t.Fatal("a saddled permanent reads solved")
	}
	s.unsaddleAll(sc.k)
	if s.matchPerm(sc.k, saddled, osl, 0, NoSlot) {
		t.Fatal("the saddled designation survived the turn-begin clear")
	}
}
