package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// K:Phasing (CR 702.26a): a permanent with it phases out as its controller's
// untap step begins and phases in at the next one, an Aura on it going out
// and coming back with it (CR 702.26g), played decision by decision against
// gorge over the phasing pool.

var phasingDecks = [2][]cardset.Entry{
	{
		{Name: "Breezekeeper", Count: 4}, {Name: "Merfolk Raiders", Count: 4}, {Name: "Teferi's Drake", Count: 4},
		{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Claustrophobia", Count: 4}, {Name: "Island", Count: 20},
	},
	{
		{Name: "Tolarian Drake", Count: 4}, {Name: "Sandbar Crocodile", Count: 4}, {Name: "Merfolk of the Pearl Trident", Count: 4},
		{Name: "Giant Octopus", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 8},
	},
}

// phasingScan counts the phase-outs (Amount 1, the keyword half, the pool's
// only source) and the phase-ins (Amount -1).
func phasingScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind == events.PhaseOut {
			if ev.Amount > 0 {
				n["out"]++
			} else {
				n["in"]++
			}
		}
	}
}

func TestPhasingLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "phasing", phasingDecks, 200, phasingScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"out", "in"} {
		if branch[want] < 500 {
			t.Errorf("%s %d, want at least 500", want, branch[want])
		}
	}
}
