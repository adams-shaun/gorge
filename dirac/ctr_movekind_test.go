package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMoveCounterAll: OpMoveCounterAll moves every counter kind of its whole
// count onto the destination; the regeneration shield is no counter and
// never moves; the same permanent moves nothing.
func TestMoveCounterAll(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	s := sc.s
	a, b := sc.perm(id["Savannah Lions"], 0), sc.perm(id["Savannah Lions"], 0)
	pa := &s.perms[sc.slot(a)]
	pa.addCtrN(s, int(kop.CtrP1P1), 3)
	pa.addCtrN(s, int(kop.CtrLoyalty), 2)
	pa.addCtrN(s, int(kop.CtrShield), 1) // the regen marker: gorge's InternalCounterMarker, no counter to move
	s.res.e.ctrl, s.res.e.obj, s.res.e.srcVer = 0, a, s.objs[a].ver
	s.res.e.trigObj = b
	in := kop.Instr{Op: kop.OpMoveCounterAll, Obj: kop.RefTriggerObj, C: int16(kop.RefSelf)}
	s.exec(sc.k, &in)
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 0 || s.perms[sc.slot(a)].ctrN(s, int(kop.CtrLoyalty)) != 0 {
		t.Fatal("every kind should leave the origin")
	}
	if s.perms[sc.slot(a)].ctrN(s, int(kop.CtrShield)) != 1 {
		t.Fatal("the regeneration shield moved")
	}
	if s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 3 || s.perms[sc.slot(b)].ctrN(s, int(kop.CtrLoyalty)) != 2 {
		t.Fatal("every kind should reach the destination")
	}
	self := in
	self.Obj = kop.RefSelf
	s.exec(sc.k, &self)
	if s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 3 {
		t.Fatal("onto itself: counters changed")
	}
}

// TestMoveCounterKind: OpMoveCounterKind asks the kind only when the origin
// holds more than one (a lone kind moves unasked, none moves nothing), and
// moves one counter of the answered kind.
func TestMoveCounterKind(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 4}})
	s := sc.s
	a, b := sc.perm(id["Savannah Lions"], 0), sc.perm(id["Savannah Lions"], 0)
	s.res.e.ctrl, s.res.e.obj, s.res.e.srcVer = 0, a, s.objs[a].ver
	s.res.e.trigObj = b
	in := kop.Instr{Op: kop.OpMoveCounterKind, Obj: kop.RefTriggerObj, C: int16(kop.RefSelf), A: kop.Const(1)}
	// The ask's options and answer read the running program (prog at res.pc,
	// dirac/ctr_movekind.go): park the op in the Plains table card's ability
	// zero and put the resolution there (the act_rearrange_test.go pattern).
	progCard := id["Plains"]
	sc.k.t.Cards[progCard].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}
	s.res.e.card, s.res.e.ab, s.res.pc = progCard, 0, 0

	// no counter: nothing moves, no ask.
	s.exec(sc.k, &in)
	if s.res.ask == raMoveCounterKind {
		t.Fatal("asked with no counter on the origin")
	}

	// one kind: taken unasked.
	s.perms[sc.slot(a)].addCtrN(s, int(kop.CtrP1P1), 2)
	s.exec(sc.k, &in)
	if s.res.ask == raMoveCounterKind {
		t.Fatal("asked with a lone kind")
	}
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 1 || s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("the lone kind should move unasked")
	}

	// two kinds: the ability's controller picks; the answered kind moves.
	s.perms[sc.slot(a)].addCtrN(s, int(kop.CtrLoyalty), 2)
	s.res.ask, s.res.askP = 0, 0
	// A suspension is (ok=false, done=true): the op consumed itself and
	// waits on res.ask (dirac/interp.go, the OpMoveCounterAny convention).
	if ok, done := s.execColdOps(sc.k, &in); !done || ok {
		t.Fatalf("exec = %v %v, want suspended", ok, done)
	}
	if s.res.ask != raMoveCounterKind {
		t.Fatalf("ask = %d, want raMoveCounterKind", s.res.ask)
	}
	var d Decision
	s.moveCounterKindOptions(sc.k, &d)
	if d.Kind != DkChoose || d.Sub != ChMoveCounterKind || d.Player != 0 {
		t.Fatalf("decision = %+v, want a ChMoveCounterKind pick", d)
	}
	modes := map[uint8]bool{}
	for i := uint16(0); i < d.N; i++ {
		modes[d.Options[i].Mode()] = true
	}
	if !modes[uint8(kop.CtrP1P1)] || !modes[uint8(kop.CtrLoyalty)] || d.N != 2 {
		t.Fatalf("options = %v, want one per held kind", d.Options[:d.N])
	}
	s.moveCounterKindAnswer(sc.k, MakeAction(AkChoose, 0, 0, uint8(kop.CtrLoyalty)))
	if s.perms[sc.slot(a)].ctrN(s, int(kop.CtrLoyalty)) != 1 || s.perms[sc.slot(b)].ctrN(s, int(kop.CtrLoyalty)) != 1 {
		t.Fatal("the answered kind should move one")
	}
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("the unchosen kind moved")
	}
}

// TestMoveCounterKindAskPlayer: the kind pick is posed to the controller of
// the ability, not the origin's controller (gorge effMoveCounter poses the
// "move_counter_kind" KChoose to c.Controller, engine/effects/counters.go):
// Rikku, Resourceful Guardian's Any move runs on the opponent's creature.
func TestMoveCounterKindAskPlayer(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	s := sc.s
	opp := sc.perm(id["Savannah Lions"], 1) // the origin: an opponent's permanent
	dst := sc.perm(id["Savannah Lions"], 0) // the destination: ours
	po := &s.perms[sc.slot(opp)]
	po.addCtrN(s, int(kop.CtrP1P1), 1)
	po.addCtrN(s, int(kop.CtrLoyalty), 1)
	s.res.e.ctrl, s.res.e.obj, s.res.e.srcVer = 0, opp, s.objs[opp].ver
	s.res.e.trigObj = dst
	in := kop.Instr{Op: kop.OpMoveCounterKind, Obj: kop.RefTriggerObj, C: int16(kop.RefSelf), A: kop.Const(1)}
	progCard := id["Plains"]
	sc.k.t.Cards[progCard].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}
	s.res.e.card, s.res.e.ab, s.res.pc = progCard, 0, 0
	if ok, done := s.execColdOps(sc.k, &in); !done || ok {
		t.Fatalf("exec = %v %v, want suspended", ok, done)
	}
	if s.res.ask != raMoveCounterKind {
		t.Fatalf("ask = %d, want raMoveCounterKind", s.res.ask)
	}
	if s.res.askP != 0 {
		t.Fatalf("askP = %d, want the ability controller 0 (gorge c.Controller), not the origin's controller 1", s.res.askP)
	}
	// The opponent's answered kind moves onto our destination.
	s.moveCounterKindAnswer(sc.k, MakeAction(AkChoose, 0, 0, uint8(kop.CtrLoyalty)))
	if s.perms[sc.slot(opp)].ctrN(s, int(kop.CtrLoyalty)) != 0 || s.perms[sc.slot(dst)].ctrN(s, int(kop.CtrLoyalty)) != 1 {
		t.Fatal("the answered kind should move from the opponent's origin")
	}
	if s.perms[sc.slot(opp)].ctrN(s, int(kop.CtrP1P1)) != 1 {
		t.Fatal("the unchosen kind moved")
	}
}
