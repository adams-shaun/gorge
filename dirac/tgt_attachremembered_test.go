package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttachObj: OpAttachObj attaches the object in Ref C (here the
// trigger's object, an Equipment) to Obj (the source creature); a gone
// object or the same permanent attaches nothing.
func TestAttachObj(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Goldvein Pick", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}})
	s := sc.s
	lions := sc.perm(id["Savannah Lions"], 0)
	other := sc.perm(id["Savannah Lions"], 0)
	pick := sc.perm(id["Goldvein Pick"], 0)
	s.res.e.ctrl, s.res.e.obj, s.res.e.card = 0, lions, id["Savannah Lions"]
	s.res.e.srcVer = s.objs[lions].ver
	s.res.e.trigObj = pick
	in := kop.Instr{Op: kop.OpAttachObj, Obj: kop.RefSelf, C: int16(kop.RefTriggerObj)}
	s.exec(sc.k, &in)
	if at := s.perms[sc.slot(pick)].attached; at != sc.slot(lions) {
		t.Fatalf("Pick attached to slot %d, want the source's %d", at, sc.slot(lions))
	}
	self := kop.Instr{Op: kop.OpAttachObj, Obj: kop.RefSelf, C: int16(kop.RefSelf)}
	s.exec(sc.k, &self)
	if s.perms[sc.slot(lions)].attached != NoSlot {
		t.Fatal("the source was attached to itself")
	}
	s.res.e.obj, s.res.e.srcVer = other, s.objs[other].ver
	s.exec(sc.k, &in)
	if at := s.perms[sc.slot(pick)].attached; at != sc.slot(other) {
		t.Fatalf("Pick attached to slot %d, want the other Lions' %d", at, sc.slot(other))
	}
}
