package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCtrlMovedSeesExpiryRevert: a slot whose only control record ends in a
// pass leaves fxM before the revert moves its controller, so ctrlMoved must
// read every occupied slot or the refresh gate in ctrlSBA misses the move
// (a control Aura destroyed mid-turn under a controller-keyed override static).
func TestCtrlMovedSeesExpiryRevert(t *testing.T) {
	l := newL5(t)
	k := l.sc.k
	s := l.sc.s
	w := l.sc.perm(l.id["Ambush Wolf"], 0)
	src := l.sc.perm(l.id["Burglar Rat"], 1)
	l.gain(w, 1, kop.DurWhileSource, src)
	if l.ctrl(w) != 1 || !s.fxM.has(l.sc.slot(w)) {
		t.Fatalf("precondition: steal must move control and track a record (ctrl %d)", l.ctrl(w))
	}
	var prev [MaxPerm]uint8
	for m := s.occ; m.any(); m = m.rest() {
		prev[m.first()] = s.perms[m.first()].ctrl
	}
	s.removePerm(k, l.sc.slot(src), kop.ZoneGraveyard, false)
	s.ctrlExpire(k, false)
	if l.ctrl(w) != 0 {
		t.Fatalf("precondition: the record ending must revert the wolf to 0, got %d", l.ctrl(w))
	}
	if s.fxM.has(l.sc.slot(w)) {
		t.Fatal("precondition: the slot must have left fxM")
	}
	if !s.ctrlMoved(&prev) {
		t.Fatal("ctrlMoved missed the expiry revert")
	}
}
