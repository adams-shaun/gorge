package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// departedOpponentLifeRepro is a four-seat hosted-cmdr pod (kfuzz -kernel
// hosted-cmdr -seats 4 -policy heuristic, pod 148): combat damage kills a
// player, then Neheb, the Eternal adds {R} for each 1 life its controller's
// opponents lost this turn. The life the departed player lost counts (Forge
// sums every registered opponent); gorge summed the living ones and added no
// mana.
const departedOpponentLifeRepro = "departed_opponent_life_loss_counts_four_seat_pod"

// departedOpponentLifeAt is the decision the recording diverged at.
const departedOpponentLifeAt = 1526

func TestHostedCmdrDepartedOpponentLifeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	lost := false
	res := hostedCmdrPodReplay(t, departedOpponentLifeRepro, func(g *GorgeSide) {
		for i := range g.E.L.Events {
			if g.E.L.Events[i].Kind == events.PlayerLost {
				lost = true
			}
		}
	})
	if !lost {
		t.Fatalf("no player left the game in %d decisions", res.Decisions)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= departedOpponentLifeAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, departedOpponentLifeAt)
	}
}
