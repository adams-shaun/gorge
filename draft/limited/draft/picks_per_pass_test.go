package draft

import (
	"errors"
	"testing"
)

// TestNewPodPicksTwoPerPass drives a Commander Legends shaped pod — 4 seats,
// 3 rounds of 20-card packs, two cards per pass — and checks that a pack
// stays with a seat for two picks, travels in the round's direction, and
// leaves every seat with 60 cards.
func TestNewPodPicksTwoPerPass(t *testing.T) {
	p, err := NewPodPicks(5, mkPacksN(4, 3, 20), 2)
	if err != nil {
		t.Fatalf("NewPodPicks: %v", err)
	}
	if p.PicksPerPass() != 2 {
		t.Fatalf("PicksPerPass = %d, want 2", p.PicksPerPass())
	}
	// Seat 0's first pick leaves its own pack at its queue head.
	if err := p.Pick(0, 1, "r1s0-a"); err != nil {
		t.Fatalf("first pick: %v", err)
	}
	v, ok := p.Next(0)
	if !ok || v.PickNo != 2 || len(v.Pack) != 19 || v.Pack[0].ID != "r1s0-b" {
		t.Fatalf("after one pick seat 0 sees pick %d of a %d-card pack (ok %v), want pick 2 of the same 19-card pack", v.PickNo, len(v.Pack), ok)
	}
	if p.QueueLen(1) != 1 {
		t.Fatalf("seat 1 holds %d packs after seat 0's first pick, want only its own", p.QueueLen(1))
	}
	if err := p.Pick(0, 2, "r1s0-b"); err != nil {
		t.Fatalf("second pick: %v", err)
	}
	if p.QueueLen(0) != 0 || p.QueueLen(1) != 2 {
		t.Fatalf("after two picks seat 0 holds %d packs and seat 1 %d, want 0 and 2", p.QueueLen(0), p.QueueLen(1))
	}
	driveAll(t, p)
	for s := 0; s < 4; s++ {
		if got := len(p.Pool(s)); got != 60 {
			t.Fatalf("seat %d pool = %d cards, want 60", s, got)
		}
	}
	has := func(seat int, id string) bool {
		for _, c := range p.Pool(seat) {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	// Round 1 passes left: seat 1 takes the third and fourth cards of
	// seat 0's pack. Round 2 passes right: seat 3 takes them.
	for _, c := range []struct {
		seat int
		id   string
	}{{0, "r1s0-b"}, {1, "r1s0-c"}, {1, "r1s0-d"}, {3, "r2s0-c"}, {3, "r2s0-d"}, {1, "r3s0-d"}} {
		if !has(c.seat, c.id) {
			t.Errorf("seat %d did not take %s", c.seat, c.id)
		}
	}
	if _, err := NewPodPicks(5, mkPacksN(4, 3, 20), 0); !errors.Is(err, ErrPodShape) {
		t.Fatalf("0 picks per pass: err %v, want ErrPodShape", err)
	}
}
