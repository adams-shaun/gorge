package dirac

import "testing"

// TestSeatHelpers holds the seat helpers to their meaning at whatever
// seat count a game has: nextSeat walks every seat once in turn order,
// SeatIx is the identity on seats, IsOpponent is "every other seat", and
// PlayerCode and CodeSeat are inverse for the seats that have a target
// code: every seat has one in either size class.
func TestSeatHelpers(t *testing.T) {
	var duel, pod State
	duel.g.nseat, pod.g.nseat = 2, MaxSeats
	for p := uint8(0); p < 2; p++ {
		if got := duel.nextSeat(p); got != 1-p {
			t.Errorf("a duel's nextSeat(%d) = %d, want %d", p, got, 1-p)
		}
	}
	s := &pod
	for p := uint8(0); p < MaxSeats; p++ {
		if got, want := s.nextSeat(p), (p+1)%MaxSeats; got != want {
			t.Errorf("s.nextSeat(%d) = %d, want %d", p, got, want)
		}
		if SeatIx(p) != p {
			t.Errorf("SeatIx(%d) = %d", p, SeatIx(p))
		}
		if CodeSeat(PlayerCode(p)) != p || !isPlayerTarget(PlayerCode(p)) {
			t.Errorf("PlayerCode/CodeSeat(%d) do not round-trip", p)
		}
		seen, opps := uint8(0), 0
		for q := s.nextSeat(p); q != p && seen < MaxSeats; q = s.nextSeat(q) {
			seen++
			if !IsOpponent(p, q) {
				t.Errorf("IsOpponent(%d, %d) = false", p, q)
			}
			opps++
		}
		if opps != MaxSeats-1 || IsOpponent(p, p) {
			t.Errorf("seat %d: %d opponents in turn order, want %d", p, opps, MaxSeats-1)
		}
	}
}
