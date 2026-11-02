package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMoveCounter: OpMoveCounter moves N (or, A < 0, all) counters of one
// kind from the origin onto the destination; the same permanent moves
// nothing.
func TestMoveCounter(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	s := sc.s
	a, b := sc.perm(id["Savannah Lions"], 0), sc.perm(id["Savannah Lions"], 0)
	s.perms[sc.slot(a)].addCtrN(s, int(kop.CtrP1P1), 3)
	s.res.e.ctrl, s.res.e.obj, s.res.e.srcVer = 0, a, s.objs[a].ver
	s.res.e.trigObj = b
	one := kop.Instr{Op: kop.OpMoveCounter, Obj: kop.RefTriggerObj, C: int16(kop.RefSelf), A: kop.Const(1), B: kop.Const(int(kop.CtrP1P1))}
	s.exec(sc.k, &one)
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 2 || s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("moving one: want 2 and 1")
	}
	all := one
	all.A = kop.Const(-1)
	s.exec(sc.k, &all)
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 0 || s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 3 {
		t.Fatal("moving all: want 0 and 3")
	}
	self := one
	self.Obj = kop.RefSelf
	s.exec(sc.k, &self)
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Fatal("onto itself: counters changed")
	}
}
