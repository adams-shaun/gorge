package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// ventureEliminatedSeeds are four-seat venture pod games where an
// UntilYourNextTurn override (Twisted Caverns' "can't attack") outlives its
// eliminated controller in the kernel. Seeds 58, 64 and 344 gorge ends at the
// boundary it froze when the controller left; seed 189 pins that the end
// follows that frozen boundary rather than the loss itself.
var ventureEliminatedSeeds = []uint64{58, 64, 344, 189}

// TestVentureEliminatedControllerRepro: an override lasting until its
// controller's next turn ends at the boundary gorge froze when the controller
// was eliminated (layers_effect.go rescheduleNextTurnBoundaries).
func TestVentureEliminatedControllerRepro(t *testing.T) {
	p := newVenturePod(t)
	for _, seed := range ventureEliminatedSeeds {
		res, _, _ := p.play(t, seed, 40)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
	}
}

// TestVentureOvNTMutation: a lost controller's next-turn records never end
// (the kernel before the frozen-boundary counter), so seed 58 must diverge.
func TestVentureOvNTMutation(t *testing.T) {
	p := newVenturePod(t)
	dirac.DebugOvNTKeepLost = true
	defer func() { dirac.DebugOvNTKeepLost = false }()
	res, _, _ := p.play(t, 58, 40)
	if res.Div == nil {
		t.Fatal("never ending a lost controller's record diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}
