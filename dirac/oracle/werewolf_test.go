package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Innistrad werewolf conditions (CR 603.4), lowered to a CondValue over
// kop.ExSpellsCastLastTurn: a front face transforms at an upkeep if no spells
// were cast last turn (by anyone), a back face (and Call of the Full Moon's
// sacrifice) if a player cast two or more. Played decision by decision
// against gorge over the werewolf pool; Shock and Grizzly Bears make the
// turns with spells, land-only draws the turns without.

var werewolfDecks = [2][]cardset.Entry{
	{
		{Name: "Reckless Waif", Count: 3}, {Name: "Village Ironsmith", Count: 3}, {Name: "Hanweir Watchkeep", Count: 3},
		{Name: "Kruin Outlaw", Count: 2}, {Name: "Hinterland Hermit", Count: 3}, {Name: "Instigator Gang", Count: 2},
		{Name: "Call of the Full Moon", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Gatstaf Shepherd", Count: 3}, {Name: "Villagers of Estwald", Count: 3}, {Name: "Mayor of Avabruck", Count: 2},
		{Name: "Daybreak Ranger", Count: 2}, {Name: "Scorned Villager", Count: 3}, {Name: "Solitary Hunter", Count: 2},
		{Name: "Huntmaster of the Fells", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 7},
	},
}

// werewolfScan counts the faces turned over (FlipFace Amount 1 the back
// face, 0 the front) and the Call of the Full Moon sacrifices.
func werewolfScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind == events.FlipFace {
			if ev.Amount == 1 {
				n["transform"]++
			} else {
				n["untransform"]++
			}
		}
	}
}

func TestWerewolfLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "werewolf", werewolfDecks, 200, werewolfScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"transform", "untransform"} {
		if branch[want] < 20 {
			t.Errorf("%s %d, want at least 20", want, branch[want])
		}
	}
}
