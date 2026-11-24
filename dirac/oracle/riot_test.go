package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// K:Riot (CR 702.136a), lowered from the printed keyword: the creature enters
// with its controller's choice of a +1/+1 counter or haste. Played decision by
// decision against gorge over the riot pool; the random policy takes both
// halves, and combat kills creatures so the elections keep coming. Every
// carrier enters by cast (the pre-entry ask): a NON-cast entry of a printed
// riot carrier diverges on the projection — gorge parks the battlefield move
// on the ask, the kernel's queued ask runs after the core enter has placed
// the permanent — the same latent shape the sanctioned Unleash model has
// (kw_unleash.go), so no reanimation or blink joins this pool (repro'd with
// Zombify).

var riotDecks = [2][]cardset.Entry{
	{
		{Name: "Ghor-Clan Wrecker", Count: 4}, {Name: "Zhur-Taa Goblin", Count: 4},
		{Name: "Burning-Tree Vandal", Count: 4}, {Name: "Rampaging Rendhorn", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Frenzied Arynx", Count: 4}, {Name: "Wrecking Beast", Count: 4},
		{Name: "Gruul Spellbreaker", Count: 4}, {Name: "Gruul Beastmaster", Count: 4},
		{Name: "Scathe Zombies", Count: 4}, {Name: "Clamor Shaman", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
}

// riotScan counts the as-enters elections by their answer (the pool's only
// "riot" Choose source) and the +1/+1 counters put (the counter half; the
// pool has no other +1/+1 counter source).
func riotScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch {
		case ev.Kind == events.Choose && ev.Counter == "riot":
			n["riotask"]++
			if ev.Text == "counter" {
				n["riotscounter"]++
			} else if ev.Text == "haste" {
				n["riotshaste"]++
			}
		case ev.Kind == events.CounterChange && ev.Counter == "P1P1" && ev.Amount > 0:
			n["counters"]++
		}
	}
}

func TestRiotLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "riot", riotDecks, 200, riotScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"riotask", "riotscounter", "riotshaste", "counters"} {
		if branch[want] < 100 {
			t.Errorf("%s %d, want at least 100", want, branch[want])
		}
	}
}
