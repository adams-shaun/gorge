package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Curator Beastie's "Colorless creatures you control enter with two additional
// +1/+1 counters" applies to a face-down manifest: CR 708.2 makes it a
// colourless 2/2 creature as it enters (CR 614.12), whatever the card is.
// gorge matched the replacement's ValidCard$ against the face-up card (an
// Island is no creature) and left the manifest a bare 2/2 where dirac gave
// it the counters (a 4/4); p4s pod 21 diverged on it. Four-seat pods of
// Beastie decks must play in lockstep, and some game must give a face-down
// creature its +1/+1 counters.
var manifestColorlessDeck = []cardset.Entry{
	{Name: "Curator Beastie", Count: 10}, {Name: "Sol Ring", Count: 4},
	{Name: "Forest", Count: 26},
}

func TestFourSeatManifestColorlessLockstep(t *testing.T) {
	t.Parallel()
	d := manifestColorlessDeck
	best := 0
	tally := func(g *state.Game) int {
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				o := g.Obj(id)
				if o == nil || !o.FaceDown {
					continue
				}
				for _, c := range o.Counters {
					if c.Kind == "P1P1" && c.N >= 2 {
						best = 1
					}
				}
			}
		}
		return best
	}
	got := cedhPodRun(t, "manifestcolorless", [][]cardset.Entry{d, d, d, d}, []string{"Curator Beastie"}, 24, tally)
	if got == 0 {
		t.Fatal("no game gave a manifested creature Curator Beastie's counters")
	}
	t.Logf("%d games with a countered manifest", got)
}
