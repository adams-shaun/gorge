package dirac

import "testing"

// TestOppsLostLifeAnyHitNibble: ExOppsLostLife N 2 (Tymna the Weaver) reads
// bit 4+q of cdLeg, the legendary read (N 1) bit q, and the life-lost read
// neither; the turn start clears the byte, both nibbles.
func TestOppsLostLifeAnyHitNibble(t *testing.T) {
	var s State
	s.g.nseat = 2
	s.g.cdLeg = 1 << (4 + 1) // seat 1 dealt combat damage by any source, no legend
	if got := s.oppsLostLife(0, 2); got != 1 {
		t.Fatalf("any-hit read for seat 0 = %d, want 1 (seat 1's high bit)", got)
	}
	if got := s.oppsLostLife(1, 2); got != 0 {
		t.Fatalf("any-hit read for seat 1 = %d, want 0 (its own bit is not an opponent's)", got)
	}
	if got := s.oppsLostLife(0, 1); got != 0 {
		t.Fatalf("legendary read = %d, want 0 (high nibble ignored)", got)
	}
	if got := s.oppsLostLife(0, 0); got != 0 {
		t.Fatalf("life-lost read = %d, want 0", got)
	}
	s.g.cdLeg = 1 << 1 // a legendary hit only
	if got := s.oppsLostLife(0, 2); got != 0 {
		t.Fatalf("any-hit read = %d, want 0 (low nibble ignored)", got)
	}
	if got := s.oppsLostLife(0, 1); got != 1 {
		t.Fatalf("legendary read = %d, want 1", got)
	}
	s.g.cdLeg = 0xF0 | 0x0F
	s.turnTalliesNewTurn(&prep{})
	if s.g.cdLeg != 0 {
		t.Fatalf("cdLeg = %#x after the turn start, want 0", s.g.cdLeg)
	}
}
