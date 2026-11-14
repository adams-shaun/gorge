//go:build dirac_mp

package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnteredLedgerHoldsNinetySix: the four-seat ledger records maxEntered
// (96) zone entries in a turn, past the 64 that stalled 2.3% of four-seat
// 100-card MBC pods, each entry's object, destination and origin read back
// from its four packed bytes, and the next entry stalls StallEntered.
func TestEnteredLedgerHoldsNinetySix(t *testing.T) {
	if maxEntered != 96 || unsafe.Sizeof(zoneEntry{}) != 4 {
		t.Fatalf("maxEntered %d, entry %d bytes: want 96 entries of 4 bytes", maxEntered, unsafe.Sizeof(zoneEntry{}))
	}
	s := new(State)
	var ring [maxEntered]zoneEntry
	s.cold.entered = ring[:]
	s.g.enteredOn, s.turn = true, 1
	zones := [...]kop.Zone{kop.ZoneGraveyard, kop.ZoneExile, kop.ZoneHand, kop.ZoneLibrary, kop.ZoneCommand}
	for i := 0; i < maxEntered; i++ {
		s.noteEnter(Obj(300+i), zones[i%len(zones)], zones[(i+1)%len(zones)], i%2 == 0)
	}
	if s.Stalled() || s.g.enteredFull || int(s.g.enteredN) != maxEntered {
		t.Fatalf("after %d entries: stalled %v, full %v, n %d", maxEntered, s.Stalled(), s.g.enteredFull, s.g.enteredN)
	}
	for i := 0; i < maxEntered; i++ {
		e := &s.cold.entered[i]
		if e.o != Obj(300+i) || e.to() != uint8(zones[i%len(zones)]) || e.from() != uint8(zones[(i+1)%len(zones)]) || (e.flags&enteredWasCreature != 0) != (i%2 == 0) {
			t.Fatalf("entry %d reads o %d to %d from %d flags %d", i, e.o, e.to(), e.from(), e.flags)
		}
	}
	s.noteEnter(Obj(7), kop.ZoneGraveyard, kop.ZoneBattlefield, false)
	if !s.g.enteredFull || s.StallReason() != StallEntered {
		t.Fatalf("entry %d: full %v, stall %s; want StallEntered", maxEntered+1, s.g.enteredFull, StallName(s.StallReason()))
	}
}
