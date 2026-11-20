package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLifeLostTargetControllerAnySeats: ExLifeLostThisTurn with the target
// controller ref reads the targeted player's own tally (CR 119.3, 119.4), not
// the sum over the controller's opponents. The target has lost 3 this turn
// and another opponent 5: the ref reads 3, the opponent sum 8.
func TestLifeLostTargetControllerAnySeats(t *testing.T) {
	_, sc, _ := oppScen(t, -1)
	s, k := sc.s, sc.k
	s.pl[0].lifeLost = 1
	s.pl[2].lifeLost = 3
	s.pl[3].lifeLost = 5
	s.res.e.tgt[0] = PlayerCode(2) // the target is seat 2 (an opponent)
	ctx := evalCtx{src: NoObj, ctrl: 0, e: &s.res.e}

	// preconditions: the compared figures differ and the target is on the
	// resolving entry's target slot the ref reads
	if got, want := s.oppsLifeLost(0), 8; got != want {
		t.Fatalf("opponents lost %d life, want %d", got, want)
	}
	if got := s.tgtCode(0); !isPlayerOrNone(got) || CodeSeat(got) != 2 {
		t.Fatalf("target 0 is %v, want player 2", got)
	}

	if got := s.lifeLostOf(k, kop.RefTargetController, ctx); got != 3 {
		t.Errorf("targeted player's life lost = %d, want 3 (not the opponents' 8)", got)
	}
	// the ref-less form still reads the controller's own tally
	if got := s.lifeLostOf(k, kop.RefNone, ctx); got != 1 {
		t.Errorf("controller's life lost = %d, want 1", got)
	}
}
