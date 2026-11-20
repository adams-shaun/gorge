package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestNonRemExclReadsTargetLKI pins the new Defined$ Targeted spelling of
// CopyPermanent Controller$ NonTargetedController: the copied permanent is
// exiled before the copy walk begins, so the excluded seat is the controller
// captured with the target (CR 608.2h, 400.7), not the owner the live card
// was reset to.
func TestNonRemExclReadsTargetLKI(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Treetop Sentries", Count: 1}, {Name: "Plains", Count: 8}})
	s, k := sc.s, sc.k
	o := sc.perm(id["Treetop Sentries"], 0) // owned by seat 0 ...
	s.perms[s.objs[o].slot].ctrl = 2        // ... controlled by seat 2 when targeted
	s.res.e.ctrl = 0
	s.res.e.tgt[0] = TCode(o)
	s.res.e.tver[0] = s.objs[o].ver
	s.snapResTarget(o)
	if s.g.tli&tliIdx == 0 {
		t.Fatal("precondition: no target LKI snapshot")
	}
	if got := s.ctrlOfObj(o); got != 2 {
		t.Fatalf("precondition: battlefield controller = %d, want 2", got)
	}

	s.leave(k, s.objs[o].slot, kop.ZoneExile, false)
	if s.objs[o].zone != kop.ZoneExile {
		t.Fatalf("target zone = %v, want exile", s.objs[o].zone)
	}
	if got := s.objs[o].owner(); got != 0 {
		t.Fatalf("precondition: owner = %d, want 0 (the wrong live answer)", got)
	}
	in := &kop.Instr{Obj: kop.RefTarget0}
	if got := s.nonRemExcl(in); got != 2 {
		t.Fatalf("nonRemExcl = %d, want the last-known controller 2", got)
	}
}
