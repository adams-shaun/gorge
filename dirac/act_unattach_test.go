package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnattachExt: ExtUnattach over the resolving source detaches an Equipment
// from its bearer (gorge effUnattach's Unattached fold: AttachedTo cleared),
// leaves it on the battlefield, and is a no-op for an unattached source.
func TestUnattachExt(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Goldvein Pick", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}})
	s := sc.s
	lions := sc.perm(id["Savannah Lions"], 0)
	pick := sc.perm(id["Goldvein Pick"], 0)
	pickSl := sc.slot(pick)
	in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtUnattach)), Obj: kop.RefSelf}
	s.res.e.ctrl, s.res.e.obj, s.res.e.card = 0, pick, id["Goldvein Pick"]
	s.res.e.srcVer = s.objs[pick].ver

	// the precondition the assertion rests on: the Pick is attached
	s.perms[pickSl].attached = sc.slot(lions)
	s.exec(sc.k, &in)
	if at := s.perms[pickSl].attached; at != NoSlot {
		t.Fatalf("Pick still attached to slot %d after Unattach", at)
	}
	if s.objs[pick].zone != kop.ZoneBattlefield {
		t.Fatalf("the detached Pick left the battlefield (zone %d)", s.objs[pick].zone)
	}

	// an unattached source: nothing to do, nothing breaks
	s.exec(sc.k, &in)
	if at := s.perms[pickSl].attached; at != NoSlot {
		t.Fatalf("Pick attached to slot %d after a no-op Unattach", at)
	}

	// the lockstep mutation keeps the attachment
	s.perms[pickSl].attached = sc.slot(lions)
	DebugUnattachNoDetach = true
	defer func() { DebugUnattachNoDetach = false }()
	s.exec(sc.k, &in)
	if at := s.perms[pickSl].attached; at != sc.slot(lions) {
		t.Fatalf("DebugUnattachNoDetach detached the Pick (attached %d)", at)
	}
}
