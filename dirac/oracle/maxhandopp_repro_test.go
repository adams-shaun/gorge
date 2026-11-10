package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestMaxHandOppReproMutation: with the opponents flag of a maximum-hand-size
// static read as "its controller" (dirac.DebugHandOppAsYou), the Winter,
// Misanthropic Guide repro diverges: the game does reach a cleanup where the
// opponents' maximum matters.
func TestMaxHandOppReproMutation(t *testing.T) {
	reg := synthReg(t)
	if div := replayStdRepro(t, reg, "std-dsk", "winter_opponent_max_hand_size_delirium", false); div != nil {
		t.Fatalf("unmutated: divergence at %d: %s %s", div.At, div.What, div.Field)
	}
	dirac.DebugHandOppAsYou = true
	defer func() { dirac.DebugHandOppAsYou = false }()
	if div := replayStdRepro(t, reg, "std-dsk", "winter_opponent_max_hand_size_delirium", false); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}
