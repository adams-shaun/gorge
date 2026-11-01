package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestChooseStarter: before the mulligan round's first decision the
// starter can be set either way and the round is posed to it first; after
// a decision, or without a mulligan round, it is refused and nothing moves.
func TestChooseStarter(t *testing.T) {
	deck := make([]CardID, 40)
	for i := range deck {
		deck[i] = vanilla.Plains
		if i%2 == 1 {
			deck[i] = vanilla.GrizzlyBears
		}
	}
	for _, p := range []uint8{0, 1} {
		var s State
		s.Init(vanilla.Table, deck, deck, 7, Config{Mulligans: 2})
		if !s.ChooseStarter(p) || s.StartingPlayer() != p {
			t.Fatalf("ChooseStarter(%d) refused or starter %d", p, s.StartingPlayer())
		}
		var d Decision
		s.StepInto(&d)
		if d.Kind != DkMulligan || d.Player != p {
			t.Fatalf("starter %d: first decision kind %d player %d", p, d.Kind, d.Player)
		}
		s.Apply(d.Options[0])
		if s.ChooseStarter(1-p) || s.StartingPlayer() != p {
			t.Fatal("ChooseStarter accepted after a decision")
		}
	}
	var s State
	s.Init(vanilla.Table, deck, deck, 7, Config{})
	if s.ChooseStarter(1-s.StartingPlayer()) || s.ChooseStarter(2) {
		t.Fatal("ChooseStarter accepted without a mulligan round")
	}
}
