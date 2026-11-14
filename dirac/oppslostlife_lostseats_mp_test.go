//go:build dirac_mp

package dirac

import "testing"

// TestOppsLostLifeSkipsLostSeats: ExOppsLostLife modes 0 (lost life), 1
// (legendary combat damage) and 2 (any combat damage) count living
// opponents only, as gorge's opponentGroup does.
func TestOppsLostLifeSkipsLostSeats(t *testing.T) {
	_, sc, _ := oppScen(t, 2)
	s := sc.s
	for q := 1; q < 4; q++ {
		s.pl[q].lifeLost = 2
	}
	s.g.cdLeg = 0xFF // every seat hit by a legend (bits 0-3) and by any source (bits 4-7)
	if !s.pl[2].lost || s.pl[2].lifeLost == 0 {
		t.Fatal("precondition: seat 2 is lost and lost life")
	}
	for mode := int16(0); mode <= 2; mode++ {
		if got := s.oppsLostLife(0, mode); got != 2 {
			t.Fatalf("mode %d: counted %d opponents, want 2 (seats 1 and 3)", mode, got)
		}
	}
}
