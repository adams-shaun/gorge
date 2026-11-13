//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSpeedFourSeats: each of four seats keeps its own speed (CR 702.179),
// and the once-per-turn latch sits above all of them.
func TestSpeedFourSeats(t *testing.T) {
	s := new(State)
	for p := uint8(0); p < MaxSeats; p++ {
		s.speedAdd(p, int(p)+1)
	}
	s.g.speed |= speedLatch
	for p := uint8(0); p < MaxSeats; p++ {
		if got, want := s.speedOf(p), min(int(p)+1, speedMaxV); got != want {
			t.Fatalf("seat %d: speed %d, want %d", p, got, want)
		}
	}
	s.speedTurn()
	if s.g.speed&speedLatch != 0 || s.speedOf(3) != speedMaxV {
		t.Fatalf("speedTurn: latch %t, seat 3 speed %d", s.g.speed&speedLatch != 0, s.speedOf(3))
	}
}

// TestFreeCastWordFourSeats: the queued free cast's caster, zone and flags
// never overlap, so the drain casts as seat 3 from the recorded zone.
func TestFreeCastWordFourSeats(t *testing.T) {
	w := seatWord(3) | seatWord(kop.ZoneExile)<<freeZoneShift | freeGY | freeCopy | freePaid | freeForget
	if c := uint8(w & freeSeatMask); c != 3 {
		t.Fatalf("caster %d, want 3", c)
	}
	if z := kop.Zone((w >> freeZoneShift) & 7); z != kop.ZoneExile {
		t.Fatalf("zone %d, want exile", z)
	}
	w &^= freeForget
	if w&freeForget != 0 || w&freePaid == 0 || uint8(w&freeSeatMask) != 3 {
		t.Fatalf("clearing ForgetPlayed disturbed the word: %#x", w)
	}
}

// TestBattleProtectorFourSeats: a battle protected by seat 2 or 3 names
// that seat (CR 310.10).
func TestBattleProtectorFourSeats(t *testing.T) {
	s := new(State)
	for p := uint8(0); p < MaxSeats; p++ {
		s.perms[0].chosen = protChosen | p
		if got, ok := s.protector(0); !ok || got != p {
			t.Fatalf("protector seat %d: got %d, %t", p, got, ok)
		}
	}
	s.perms[0].chosen = 255
	if _, ok := s.protector(0); ok {
		t.Fatal("an unset choice reads as a protector")
	}
}

// TestLeftControllerFourSeats: the entered ledger keeps a departure's
// controller whole, so "you controlled" (CR 603.10a) reads seat 2 as seat 2.
func TestLeftControllerFourSeats(t *testing.T) {
	s := new(State)
	var ring [4]zoneEntry
	s.cold.entered = ring[:]
	for p := uint8(0); p < MaxSeats; p++ {
		s.g.enteredN = 1
		ring[0] = zoneEntry{o: 7, from: uint8(kop.ZoneBattlefield), flags: enteredWasCreature}
		s.noteLeftController(7, p)
		if got := ring[0].leftController(); got != p {
			t.Fatalf("left under seat %d, ledger reads %d", p, got)
		}
		if ring[0].flags&enteredWasCreature == 0 {
			t.Fatalf("seat %d: the was-creature bit was lost", p)
		}
	}
}
