package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// K:Scavenge (CR 702.97a), lowered from gorge's kwScavenge expansion: from
// the graveyard, at sorcery speed, pay the cost and exile the card to put
// +1/+1 counters equal to its power on a target creature. Played decision by
// decision against gorge over the scavenge pool; Doom Blade and combat fill
// the graveyards, and the pool has no other +1/+1 counter source.

var scavDecks = [2][]cardset.Entry{
	{
		{Name: "Deadbridge Goliath", Count: 3}, {Name: "Drudge Beetle", Count: 3}, {Name: "Dreg Mangler", Count: 3},
		{Name: "Sewer Shambler", Count: 3}, {Name: "Slitherhead", Count: 3}, {Name: "Doom Blade", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Terrus Wurm", Count: 3}, {Name: "Thrashing Mossdog", Count: 3}, {Name: "Zanikev Locust", Count: 3},
		{Name: "Sluiceway Scorpion", Count: 3}, {Name: "Korozda Monitor", Count: 3}, {Name: "Doom Blade", Count: 4},
		{Name: "Scathe Zombies", Count: 4}, {Name: "Swamp", Count: 9}, {Name: "Forest", Count: 8},
	},
}

// scavScan counts the +1/+1 counters put on (all from scavenging here) and
// the nontoken cards exiled from a graveyard (the scavenge cost).
func scavScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.CounterChange:
			if ev.Counter == "P1P1" && ev.Amount > 0 {
				n["counters"]++
				n["counterN"] += int(ev.Amount)
			}
		case events.MoveZone:
			if ev.From == state.ZGraveyard && ev.To == state.ZExile {
				n["exiled"]++
			}
		}
	}
}

func TestScavengeLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "scavenge", scavDecks, 200, scavScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"counters", "exiled"} {
		if branch[want] < 100 {
			t.Errorf("%s %d, want at least 100", want, branch[want])
		}
	}
}
