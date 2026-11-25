package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// K:Champion (CR 702.72a), lowered from gorge's kwChampion expansion: when
// the champion enters, exile another [type] you control until it leaves, or
// sacrifice it. Played decision by decision against gorge over the champion
// pool; Shock and Doom Blade remove champions, returning the exiled card.

var champDecks = [2][]cardset.Entry{
	{
		{Name: "Boggart Mob", Count: 3}, {Name: "Lightning Crafter", Count: 3}, {Name: "Nova Chaser", Count: 3},
		{Name: "Changeling Berserker", Count: 3}, {Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 3},
		{Name: "Doom Blade", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Changeling Hero", Count: 3}, {Name: "Changeling Titan", Count: 3}, {Name: "Thoughtweft Trio", Count: 3},
		{Name: "Wren's Run Packmaster", Count: 3}, {Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 8},
	},
}

// champScan counts battlefield-to-exile moves (the champion's exile), their
// returns, and champions put into a graveyard from the battlefield (a
// sacrificed champion, or one Shock or Doom Blade killed).
func champScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.MoveZone:
			switch {
			case ev.From == state.ZBattlefield && ev.To == state.ZExile:
				n["exiled"]++
			case ev.From == state.ZExile && ev.To == state.ZBattlefield:
				n["returned"]++
			case ev.From == state.ZBattlefield && ev.To == state.ZGraveyard:
				if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && champNames[o.Face().Name] {
					n["championLeft"]++
				}
			}
		}
	}
}

var champNames = map[string]bool{
	"Boggart Mob": true, "Lightning Crafter": true, "Nova Chaser": true, "Changeling Berserker": true,
	"Changeling Hero": true, "Changeling Titan": true, "Thoughtweft Trio": true, "Wren's Run Packmaster": true,
}

func TestChampionLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "champion", champDecks, 200, champScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"exiled", "returned", "championLeft"} {
		if branch[want] < 20 {
			t.Errorf("%s %d, want at least 20", want, branch[want])
		}
	}
}
