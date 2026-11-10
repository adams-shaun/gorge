package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUntapLockSourceExpiry pins the two source-departure contracts of an
// untap lock (act_untaplock.go), both matching gorge's continuousLive: a card
// source that leaves keeps its record and revives it by returning under the
// same controller, and a token source that leaves drops it for good, since its
// object is reused.
func TestUntapLockSourceExpiry(t *testing.T) {
	sc, lc := untapLockScen(t)
	s := sc.s
	tgt := sc.perm(lc, 1)

	// a card source leaves and returns under the same controller
	src := sc.perm(lc, 0)
	sc.lockOn(tgt, src, 0, kop.SkipUntapLockAny)
	if s.g.nul != 1 || !s.untapLocked(tgt, true) {
		t.Fatalf("records %d locked %v, want 1 and true", s.g.nul, s.untapLocked(tgt, true))
	}
	s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)
	if s.g.nul != 1 || s.untapLocked(tgt, true) {
		t.Fatalf("source in the graveyard: records %d locked %v, want 1 and false", s.g.nul, s.untapLocked(tgt, true))
	}
	sc.enterAs(src, 0)
	if !s.untapLocked(tgt, true) {
		t.Fatal("the card source returned under the same controller and the lock did not come back")
	}
	// ... but not under another controller
	s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)
	sc.enterAs(src, 1)
	if s.untapLocked(tgt, true) {
		t.Fatal("the card source returned under another controller and the lock is live")
	}
	s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)
	s.untapLockDrop(func(untapLock) bool { return true })

	// a token source that leaves drops its record
	tok := s.newToken(lc, 0)
	sc.enterAs(tok, 0)
	if s.objs[tok].of&ofToken == 0 {
		t.Fatal("the source is not a token")
	}
	sc.lockOn(tgt, tok, 0, kop.SkipUntapLockStep)
	if s.g.nul != 1 {
		t.Fatalf("records %d, want 1", s.g.nul)
	}
	s.leave(sc.k, sc.slot(tok), kop.ZoneGraveyard, false)
	if s.g.nul != 0 {
		t.Fatalf("records %d after the token source left, want 0", s.g.nul)
	}
}
