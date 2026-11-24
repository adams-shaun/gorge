package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// K:Vanishing (CR 702.63a), lowered from gorge's kwVanishing expansion: the
// creature enters with N time counters, loses one at each of its controller's
// upkeeps, and is sacrificed when the last is removed (a positive-to-zero
// transition). Played decision by decision against gorge over the vanishing
// pool; the two decks field the carriers beside plain creatures.

var vanishDecks = [2][]cardset.Entry{
	{
		{Name: "Keldon Marauders", Count: 4}, {Name: "Lavacore Elemental", Count: 4}, {Name: "Deep Forest Hermit", Count: 3},
		{Name: "Calciderm", Count: 3}, {Name: "Soultether Golem", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Aven Riftwatcher", Count: 4}, {Name: "Waning Wurm", Count: 4}, {Name: "Deadly Grub", Count: 4},
		{Name: "Elite Vanguard", Count: 4}, {Name: "Scathe Zombies", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

// vanishScan counts the time counters put on and removed (the pool's only
// time-counter source is the carriers) and the carriers that went from the
// battlefield to a graveyard (the last-counter sacrifice among them).
func vanishScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.CounterChange:
			if ev.Counter == "TIME" {
				if ev.Amount > 0 {
					n["timeadded"]++
				} else if ev.Amount < 0 {
					n["timeremoved"]++
				}
			}
		case events.MoveZone:
			if ev.From == state.ZBattlefield && ev.To == state.ZGraveyard {
				if o := g.E.G.Obj(ev.Obj); o != nil && o.Counter("TIME") == 0 && o.Face() != nil && !o.IsToken {
					n["gone"]++
				}
			}
		}
	}
}

func TestVanishingLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "vanishing", vanishDecks, 200, vanishScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"timeadded", "timeremoved", "gone"} {
		if branch[want] < 200 {
			t.Errorf("%s %d, want at least 200", want, branch[want])
		}
	}
}
