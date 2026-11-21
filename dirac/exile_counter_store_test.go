package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileCounterStoreVersionDrop: the counter a dies-to-exile replacement
// leaves on an exiled card is a record of that object, not of its slot. A card
// that leaves exile and comes back is a new object (CR 400.7), so the old
// record is dead: the re-entering card carries no VOID counter until some
// replacement puts one on it again.
func TestExileCounterStoreVersionDrop(t *testing.T) {
	sc, id := exileCtrScen(t)
	s, k := sc.s, sc.k
	sc.perm(id["Dauthi Voidwalker"], 0)
	bear := sc.perm(id["Grizzly Bears"], 1)
	if z := s.objs[bear].zone; z != kop.ZoneBattlefield {
		t.Fatalf("precondition: bear is in zone %d, want the battlefield", z)
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 0 {
		t.Fatalf("precondition: the battlefield bear has %d exile VOID counters", n)
	}

	s.leave(k, sc.slot(bear), kop.ZoneGraveyard, false)
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the dying bear is in zone %d, want exile", z)
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 1 {
		t.Fatalf("the exiled bear has %d VOID counters, want 1", n)
	}
	ver := s.objs[bear].ver

	// Leave exile and come back by a plain move: no replacement runs, and the
	// old counter must not attach to the new object.
	s.placeFromK(k, bear, kop.ZoneHand, false, kop.ZoneExile, false, false)
	if z := s.objs[bear].zone; z != kop.ZoneHand {
		t.Fatalf("the bear is in zone %d, want the hand", z)
	}
	if s.objs[bear].ver == ver {
		t.Fatal("precondition: leaving exile did not make a new object (version unchanged)")
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 0 {
		t.Fatalf("the bear in the hand still reports %d exile VOID counters", n)
	}
	s.placeFromK(k, bear, kop.ZoneExile, false, kop.ZoneHand, false, false)
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the bear is in zone %d, want exile again", z)
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 0 {
		t.Fatalf("the re-entered exiled bear still carries %d VOID counters, want 0", n)
	}
}
