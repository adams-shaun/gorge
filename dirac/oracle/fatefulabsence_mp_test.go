package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Fateful Absence: "Destroy target creature or planeswalker. Its controller
// investigates." (Investigate Defined$ TargetedController; CR 701.16a). The
// Clue goes to the destroyed permanent's controller, the referent a Swords to
// Plowshares life gain reads after its exile. Four-seat pods play in
// lockstep and some game gives a Clue to a player.
var fatefulAbsenceDeck = []cardset.Entry{
	{Name: "Fateful Absence", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6},
	{Name: "Plains", Count: 12}, {Name: "Forest", Count: 8},
}

func TestFourSeatFatefulAbsenceLockstep(t *testing.T) {
	t.Parallel()
	d := fatefulAbsenceDeck
	clues := 0
	tally := func(g *state.Game) int {
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.Face() != nil && strings.HasPrefix(o.Face().Name, "Clue") {
					clues = 1
				}
			}
		}
		return clues
	}
	got := cedhPodRun(t, "fatefulabsence", [][]cardset.Entry{d, d, d, d}, []string{"Fateful Absence"}, 16, tally)
	if got == 0 {
		t.Fatal("no game made a Clue")
	}
	t.Logf("%d games with a Clue", got)
}
