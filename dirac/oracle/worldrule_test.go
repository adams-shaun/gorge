package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CR 704.5k world-rule lockstep: four printed World enchantments in each
// deck, so the newest one cast puts every older one into its owner's
// graveyard (gorge rules/sba.go worldRule; kernel sba_world.go).

var worldRuleDeck = []cardset.Entry{
	{Name: "Concordant Crossroads", Count: 4}, {Name: "Storm World", Count: 4},
	{Name: "Chaosphere", Count: 4}, {Name: "Gravity Sphere", Count: 4},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
}

func TestWorldRuleLockstep(t *testing.T) {
	reg, set, decks, tbl := p3Setup(t, "worldrule", [2][]cardset.Entry{worldRuleDeck, worldRuleDeck})
	worldGames := 0
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"worldrule-a", "worldrule-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: %s", seed, res.Stall)
		}
		for _, e := range g.E.L.Events {
			if e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.Text == "world rule" {
				worldGames++
				break
			}
		}
		if d := res.Div; d != nil {
			tail := res.Actions
			if len(tail) > 6 {
				tail = tail[len(tail)-6:]
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, tail)
		}
	}
	if worldGames == 0 {
		t.Fatal("precondition: no game's gorge log has a world-rule move")
	}
	t.Logf("%d of 40 games applied the world rule", worldGames)
}
