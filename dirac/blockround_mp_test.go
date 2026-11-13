//go:build dirac_mp

package dirac

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestNSeatBlockRoundActiveLeft: the active player left the game during its
// own turn (CR 800.4a: the turn goes on), and the declare-blockers round
// still walks the defenders once, from the seat after it round to it,
// skipping departed seats, and finishes. A walk that stopped only on meeting
// the active player among the live seats never ended (a four-seat Commander
// pod hung in its declare-blockers step).
func TestNSeatBlockRoundActiveLeft(t *testing.T) {
	var decks [MaxSeats][]CardID
	for p := range decks {
		decks[p] = combatDeck()
	}
	s := new(State)
	s.InitN(v.Table, decks, 4, 1, Config{NoElide: true})
	s.turn, s.active, s.step = 3, 2, kop.StepBlockers
	s.pl[2].lost, s.pl[0].lost = true, true
	done := make(chan struct{})
	go func() {
		s.blockRound(s.k(), s.active)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the block round did not finish with the active player gone")
	}
}
