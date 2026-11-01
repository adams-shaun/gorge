package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// Neither deck has another source of the Villages' colour, and a Village's
// restricted mana cannot pay an ability (CR 106.6), so each Village activation
// is funded by Hidden Grotto's filter (CR 605.3a), never by the Village's own
// mana: its {T} is the ability's cost (CR 602.2b).
var villageFilterDecks = [2][]cardset.Entry{
	{
		{Name: "Lupinflower Village", Count: 6}, {Name: "Hidden Grotto", Count: 6}, {Name: "Forest", Count: 12},
		{Name: "Sunshower Druid", Count: 8}, {Name: "Treeguard Duo", Count: 8},
	},
	{
		{Name: "Rockface Village", Count: 6}, {Name: "Hidden Grotto", Count: 6}, {Name: "Swamp", Count: 12},
		{Name: "Raccoon Rallier", Count: 16},
	},
}

func TestVillageFundedByFilterLockstep(t *testing.T) {
	t.Parallel()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("blb")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, villageFilterDecks)
	tbl, _, _, err := set.Matchup(villageFilterDecks[0], villageFilterDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	fired := map[string]int{}
	for seed := uint64(1); seed <= 64; seed++ {
		m, err := NewMatchup(reg, [2]string{"lupinflower", "rockface"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30}
		rc.Finish = func(g *GorgeSide) {
			for i := range g.E.L.Events {
				ev := &g.E.L.Events[i]
				if ev.Kind != events.AbilityPush {
					continue
				}
				if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
					fired[ob.Face().Name]++
				}
			}
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at %d: %s %s %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		}
	}
	for _, name := range []string{"Lupinflower Village", "Rockface Village"} {
		t.Logf("%s activations: %d", name, fired[name])
		if fired[name] == 0 {
			t.Errorf("%s was never activated", name)
		}
	}
}
