package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Endless Atlas: "{2}, {T}: Draw a card. Activate only if you control three or
// more lands with the same name." (CheckSVar$ X GE3, X = Count$MostCardName
// Land.YouCtrl: gorge countMostCardName, the most lands sharing one face name;
// kernel ExValidAgg AggMostName). The mixed basics keep the count below the
// land total until one name reaches three. Four-seat pods play in lockstep
// and some game activates it (the Atlas taps only for its ability).
var endlessAtlasDeck = []cardset.Entry{
	{Name: "Endless Atlas", Count: 8}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 4},
}

func TestFourSeatEndlessAtlasLockstep(t *testing.T) {
	t.Parallel()
	d := endlessAtlasDeck
	used := 0
	tally := func(g *state.Game) int {
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.Tapped && o.Face() != nil && o.Face().Name == "Endless Atlas" {
					used = 1
				}
			}
		}
		return used
	}
	got := cedhPodRun(t, "endlessatlas", [][]cardset.Entry{d, d, d, d}, []string{"Endless Atlas"}, 16, tally)
	if got == 0 {
		t.Fatal("no game activated Endless Atlas")
	}
	t.Logf("%d games activated Endless Atlas", got)
}
