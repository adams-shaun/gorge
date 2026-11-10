package dirac

import "testing"

// TestSeatHelpers holds the seat helpers to their meaning at whatever
// MaxSeats the build has: NextSeat walks every seat once in turn order,
// SeatIx is the identity on seats, IsOpponent is "every other seat", and
// PlayerCode and CodeSeat are inverse for the seats that have a target
// code. There are two player codes in either size class, so the round trip
// is held for seats 0 and 1 only.
func TestSeatHelpers(t *testing.T) {
	for p := uint8(0); p < MaxSeats; p++ {
		if got, want := NextSeat(p), (p+1)%MaxSeats; got != want {
			t.Errorf("NextSeat(%d) = %d, want %d", p, got, want)
		}
		if SeatIx(p) != p {
			t.Errorf("SeatIx(%d) = %d", p, SeatIx(p))
		}
		if p < 2 && (CodeSeat(PlayerCode(p)) != p || !isPlayerTarget(PlayerCode(p))) {
			t.Errorf("PlayerCode/CodeSeat(%d) do not round-trip", p)
		}
		seen, opps := uint8(0), 0
		for q := NextSeat(p); q != p && seen < MaxSeats; q = NextSeat(q) {
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
