package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestExtraTurnQueueMutation: the queue's order is the rule (CR 500.7, most
// recently created first) and its mutation is visible. With
// DebugExtraTurnFIFO the oldest grant is spent first -- the wrong seat's
// turn begins, the rotation resumes one turn later, and the turn-fact
// readers follow the wrong seat.
func TestExtraTurnQueueMutation(t *testing.T) {
	var decks [MaxSeats][]CardID
	for p := 0; p < MaxSeats; p++ {
		decks[p] = vanilla.Decks[p%len(vanilla.Decks)]
	}
	s := new(State)
	s.InitN(vanilla.Table, decks, MaxSeats, 7, Config{})
	if s.turn != 1 || s.active != s.g.exBase {
		t.Fatalf("turn %d active %d exBase %d: the first turn is a normal one", s.turn, s.active, s.g.exBase)
	}
	// Two grants pending seat 0's turn: seat 2's first, seat 3's later. CR
	// 500.7 spends the later one first and inserts both after this turn,
	// so the rotation resumes after seat 0's ordinary successor.
	s.exPush(2, false, NoObj)
	s.exPush(3, false, NoObj)
	if got := s.NextTurnPlayer(); got != 3 {
		t.Fatalf("NextTurnPlayer %d, want 3 (the most recent grant)", got)
	}
	if d := s.turnsTo(3); d != 1 {
		t.Fatalf("turnsTo(3) %d, want 1", d)
	}
	if d := s.turnsTo(2); d != 2 {
		t.Fatalf("turnsTo(2) %d, want 2 (seat 3's granted turn first)", d)
	}
	s.exTake()
	if s.active != 3 || s.g.exN != 1 || s.g.exQ[0] != 2 {
		t.Fatalf("active %d pending %d (%d): seat 3's turn began, seat 2's grant waits", s.active, s.g.exN, s.g.exQ[0])
	}
	if s.g.exBase != 0 {
		t.Fatalf("exBase %d: a granted turn left the rotation base alone", s.g.exBase)
	}
	s.exTake()
	if s.active != 2 || s.g.exN != 0 {
		t.Fatalf("active %d pending %d: seat 2's turn began, the queue drained", s.active, s.g.exN)
	}
	s.exTake()
	if want := s.nextAlive(0); s.active != want || s.g.exBase != want {
		t.Fatalf("active %d exBase %d, want %d: the rotation resumed after seat 0", s.active, s.g.exBase, want)
	}

	// A grant to a seat that has lost is consumed with no turn and the next
	// grant is spent in the same cleanup: seat 2's is the later one, so the
	// consumer skips it and seat 3's turn begins here.
	s.pl[2].lost = true
	s.exPush(3, false, NoObj)
	s.exPush(2, false, NoObj)
	s.exTake()
	s.pl[2].lost = false
	if s.active != 3 || s.g.exN != 0 {
		t.Fatalf("active %d pending %d: seat 2's lost grant was skipped, seat 3's turn began", s.active, s.g.exN)
	}

	// The mutation: the oldest grant first is the wrong order, and every
	// reader follows it.
	DebugExtraTurnFIFO = true
	defer func() { DebugExtraTurnFIFO = false }()
	s.exPush(2, false, NoObj)
	s.exPush(3, false, NoObj)
	if got := s.NextTurnPlayer(); got != 2 {
		t.Fatalf("FIFO NextTurnPlayer %d, want 2: the mutation reads the oldest first", got)
	}
	if d := s.turnsTo(2); d != 1 {
		t.Fatalf("FIFO turnsTo(2) %d, want 1", d)
	}
	s.exTake()
	if s.active != 2 {
		t.Fatalf("FIFO active %d, want 2: the mutation spent the oldest grant", s.active)
	}
	if s.g.exN != 1 || s.g.exQ[0] != 3 {
		t.Fatalf("FIFO pending %d (%d): seat 3's grant waits", s.g.exN, s.g.exQ[0])
	}
}
