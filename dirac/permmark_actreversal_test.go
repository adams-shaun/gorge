package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPermMarkActivatedReversal: a reversed activation (CR 733.1) gives back
// the activation count, so the activated-this-turn mark (gorge's
// Object.ActivatedThisTurn > 0, Cut Short's Planeswalker.activated) reads
// false again; a second activation that is reversed leaves the first's mark.
func TestPermMarkActivatedReversal(t *testing.T) {
	tbl := &kop.Table{
		Filters: []kop.Filter{{Types: kop.TCreature, Pred: kop.PredPermMark, PredN: 4}},
		Exprs:   []kop.Expr{{Kind: kop.ExConst, N: 1}},
		Cards: []kop.CardDef{
			{Name: "Test Plains", Types: kop.TLand},
			{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		},
	}
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.ix[mechActTurn] == arenaNone || !k.gen.anyPermMark {
		t.Fatalf("the table's activated word did not bring in mechActTurn: gen %v", k.gen)
	}
	bear := sc.perm(1, 0)
	sl := sc.slot(bear)
	if s.mSl(k, mechActTurn, sl) {
		t.Fatal("precondition: a fresh permanent carries the activated mark")
	}
	push := func() {
		t.Helper()
		s.ctl = ctlBuild
		e := &s.bld().e
		e.kind, e.obj, e.trigObj = seAbility, bear, NoObj
		s.bld().bfl |= bflPushed
		s.bld().pushNP = uint8(s.npend)
		s.sp++
		ob := &s.objs[bear]
		ob.acts++
		s.markActivated(k, ob.slot) // what cast.go's push does beside acts++
	}
	push()
	if !s.mSl(k, mechActTurn, sl) {
		t.Fatal("precondition: the push did not set the activated mark")
	}
	s.unpushActivation(k)
	if n := s.objs[bear].activations(); n != 0 {
		t.Fatalf("the reversal left %d activations, want 0", n)
	}
	if s.mSl(k, mechActTurn, sl) {
		t.Fatal("the only activation was reversed but the activated mark still reads true")
	}

	// Two activations, the second reversed: the first still counts.
	push()
	push()
	s.unpushActivation(k)
	if n := s.objs[bear].activations(); n != 1 {
		t.Fatalf("after one reversal of two: %d activations, want 1", n)
	}
	if !s.mSl(k, mechActTurn, sl) {
		t.Fatal("reversing the second activation cleared the first's mark")
	}
}
