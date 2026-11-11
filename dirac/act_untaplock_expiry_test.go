package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUntapLockSourceExpiry pins the source-departure contract of an untap
// lock (act_untaplock.go), matching gorge's continuousLive reading the
// registration's source incarnation: a card source or a token source that
// leaves drops its record for good, and a card that returns does not revive it
// (CR 400.7, 611.2b). A lock whose source is already gone adds no record.
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
	if s.g.nul != 0 {
		t.Fatalf("source in the graveyard: records %d, want 0", s.g.nul)
	}
	sc.enterAs(src, 0)
	if s.untapLocked(tgt, true) {
		t.Fatal("the card source returned and the lock came back")
	}
	s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)

	// a source already gone when the lock resolves adds nothing
	sc.lockOn(tgt, src, 0, kop.SkipUntapLockStep)
	if s.g.nul != 0 {
		t.Fatalf("records %d from a source in the graveyard, want 0", s.g.nul)
	}

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
