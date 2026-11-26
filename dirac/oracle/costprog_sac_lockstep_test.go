package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The two-sacrifice-part cost-program lockstep: Viscerid Drone sacrifices a
// creature and a Swamp (or a snow Swamp), each part its own Sac atom asked
// with its own filter, a permanent paying one part only, and the offer
// requiring a distinct assignment (dirac/cost_progsac.go; gorge sacAsk and
// SacrificeRemainderFeasible). Four deck pairs, both surfaces. The check
// proves the games sacrifice a Swamp for the cost (no other card here moves
// a land off the battlefield), so a run that never activates cannot pass.
var costProgSacDecks = [4][2][]cardset.Entry{
	{
		{
			{Name: "Viscerid Drone", Count: 4}, {Name: "Grizzly Bears", Count: 10},
			{Name: "Swamp", Count: 10}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Grizzly Bears", Count: 14}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 10},
		},
	},
	{
		{
			{Name: "Viscerid Drone", Count: 4}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Snow-Covered Swamp", Count: 8}, {Name: "Snow-Covered Island", Count: 8}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Viscerid Drone", Count: 4}, {Name: "Grizzly Bears", Count: 12},
			{Name: "Swamp", Count: 12}, {Name: "Island", Count: 12},
		},
	},
	{
		{
			{Name: "Viscerid Drone", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Swamp", Count: 6}, {Name: "Snow-Covered Swamp", Count: 6}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 10}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8},
		},
	},
	{
		{
			{Name: "Viscerid Drone", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Swamp", Count: 8}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Viscerid Drone", Count: 4}, {Name: "Grizzly Bears", Count: 10},
			{Name: "Snow-Covered Swamp", Count: 10}, {Name: "Snow-Covered Island", Count: 10}, {Name: "Forest", Count: 6},
		},
	},
}

func TestCostProgTwoSacrificeLockstep(t *testing.T) {
	t.Parallel()
	for i, decks := range costProgSacDecks {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			reg, set, gd := lockstepPool(t, "costprogsac", decks)
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for seed := uint64(1); seed <= 24; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"costprogsac-a", "costprogsac-b"}, gd, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				saw := false
				var prev [2]int
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					g := ref.E.G
					for p := state.PlayerID(0); p < 2; p++ {
						n := 0
						for _, o := range g.Zone(state.ZBattlefield, p) {
							if ob := g.Obj(o); ob.Card != nil && len(ob.Card.Faces) != 0 {
								if name := ob.Card.Faces[0].Name; name == "Swamp" || name == "Snow-Covered Swamp" {
									n++
								}
							}
						}
						if n < prev[p] {
							saw = true // a Swamp left: the Drone's cost sacrificed it
						}
						prev[p] = n
					}
					return nil
				}
				res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\ngorge %v\nkernel %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				if saw {
					seen++
				}
			}
			if seen == 0 {
				t.Fatal("no seed sacrificed a creature and a Swamp for Viscerid Drone")
			}
			t.Logf("%d of 24 seeds sacrificed a creature and a Swamp for Viscerid Drone", seen)
		})
	}
}
