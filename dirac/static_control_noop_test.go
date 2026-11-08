package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestL5StaticOverOwnControllerThenSteal: a control static that begins on
// a permanent its controller already controls is still a layer-2 effect
// with a timestamp (CR 613.7). It registers a no-op record, so a later
// steal stacks on top of it instead of the standing static re-taking the
// permanent at the next state-based check (gorge's noop grant in
// reconcileControlStatics).
func TestL5StaticOverOwnControllerThenSteal(t *testing.T) {
	l := newL5(t)
	s, k := l.sc.s, l.sc.k
	w := l.sc.perm(l.id["Ambush Wolf"], 1)
	a := l.sc.perm(l.id["Witness Protection"], 1)
	s.attachTo(k, l.sc.slot(a), l.sc.slot(w))
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("precondition: host controller %d, want the static's own 1", got)
	}
	if got := l.records(); got != 1 {
		t.Fatalf("a static over its own controller's host registered %d records, want its 1 no-op grant", got)
	}
	l.gain(w, 0, kop.DurEOT, NoObj)
	s.sba(k)
	if got := l.ctrl(w); got != 0 {
		t.Fatalf("after a later EOT steal: controller %d, want 0 (the later steal stands)", got)
	}
	if got := l.records(); got != 2 {
		t.Fatalf("after the steal: %d control records, want 2 (the static's no-op and the steal)", got)
	}
	s.ctrlCleanup(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("after the EOT steal ended: controller %d, want 1 (the static's controller)", got)
	}
}

// TestL5SameControllerAurasLeaveWhileAttacking: two same-controller Auras
// resolve one at a time; when the earlier one leaves, the later winner's
// no-op record must hold the host instead of a transient hand-back to its
// owner, which would remove it from combat (CR 506.4; gorge's noop grant
// keeps the later Aura's want tracked).
func TestL5SameControllerAurasLeaveWhileAttacking(t *testing.T) {
	l := newL5(t)
	s, k := l.sc.s, l.sc.k
	w := l.sc.perm(l.id["Ambush Wolf"], 0)
	x := l.sc.perm(l.id["Ambush Wolf"], 0)
	a0 := l.sc.perm(l.id["Witness Protection"], 1)
	a1 := l.sc.perm(l.id["Witness Protection"], 1)
	// Park a1 on x so the first SBA does not bin it as an unattached Aura;
	// it resolves onto w after a0, giving it the later layer timestamp.
	s.attachTo(k, l.sc.slot(a1), l.sc.slot(x))
	s.attachTo(k, l.sc.slot(a0), l.sc.slot(w))
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("precondition: first Aura's controller %d, want 1", got)
	}
	if got := l.hostRecords(l.sc.slot(w)); got != 1 {
		t.Fatalf("precondition: %d records on the host after the first Aura, want 1", got)
	}
	s.attachTo(k, l.sc.slot(a1), l.sc.slot(w))
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("second same-controller Aura: controller %d, want 1", got)
	}
	if got := l.hostRecords(l.sc.slot(w)); got != 2 {
		t.Fatalf("second same-controller Aura: %d records on the host, want 2 (a0's grant and a1's no-op)", got)
	}
	sl := l.sc.slot(w)
	s.perms[sl].flags |= pfAttacking
	s.nAttackers = 1
	s.removePerm(k, l.sc.slot(a0), kop.ZoneGraveyard, false)
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("after the earlier Aura left: controller %d, want 1 (the later Aura still wins)", got)
	}
	if s.perms[sl].flags&pfAttacking == 0 || s.nAttackers != 1 {
		t.Fatalf("a transient hand-back removed the host from combat (attacking %v, nAttackers %d)",
			s.perms[sl].flags&pfAttacking != 0, s.nAttackers)
	}
	if got := l.hostRecords(sl); got != 1 {
		t.Fatalf("after the earlier Aura left: %d records on the host, want the later Aura's 1", got)
	}
}

// hostRecords counts the control records on one permanent (l5scen.records
// counts the whole state).
func (l *l5scen) hostRecords(sl Slot) int {
	n := 0
	for i := 0; i < int(l.sc.s.g.nfx); i++ {
		if r := &l.sc.s.g.fx[i]; r.kind == fxControl && r.sl == sl {
			n++
		}
	}
	return n
}
