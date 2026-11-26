package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The SubCounter<All/KIND> cost-program lockstep: each carrier's activated
// ability removes every counter of one kind from its source as the cost (a
// kop.CostAtom program paid by dirac/cost_prog.go) and its effect reads how
// many were removed (SVar$CostCountersRemoved: gorge Ctx.Paid.CountersRemoved,
// the kernel's entry amount). Four deck pairs, both surfaces (the seed's
// parity). The check proves the games remove a pile of at least one counter
// by the cost, so a run that never activates cannot pass vacuously.
var costProgSubCtrDecks = [4][2][]cardset.Entry{
	{ // Molten Hydra's damage and Essence Bottle's life
		{
			{Name: "Molten Hydra", Count: 4}, {Name: "Essence Bottle", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 14}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
		},
	},
	{ // Jar of Eyeballs' dig, Torture Chamber's and Relic Amulet's damage
		{
			{Name: "Jar of Eyeballs", Count: 4}, {Name: "Torture Chamber", Count: 4},
			{Name: "Relic Amulet", Count: 4}, {Name: "Grizzly Bears", Count: 10},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
		},
	},
	{ // Geometric Nexus' Fractal and Relic Amulet, both charged by spells
		{
			{Name: "Geometric Nexus", Count: 4}, {Name: "Relic Amulet", Count: 4},
			{Name: "Lightning Bolt", Count: 8}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Molten Hydra", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Forest", Count: 10},
		},
	},
	{ // Vish Kal's sacrifice-fed counters and Catti-brie's attack counters
		{
			{Name: "Vish Kal, Blood Arbiter", Count: 4}, {Name: "Catti-brie of Mithral Hall", Count: 4},
			{Name: "Grizzly Bears", Count: 12}, {Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
			{Name: "Forest", Count: 6},
		},
		{
			{Name: "Molten Hydra", Count: 4}, {Name: "Essence Bottle", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 14}, {Name: "Forest", Count: 6},
		},
	},
}

// costProgPile is each carrier and the counter kind its cost removes.
var costProgPile = map[string]string{
	"Molten Hydra": "P1P1", "Essence Bottle": "ELIXIR", "Jar of Eyeballs": "EYEBALL",
	"Torture Chamber": "PAIN", "Relic Amulet": "CHARGE", "Geometric Nexus": "CHARGE",
	"Vish Kal, Blood Arbiter": "P1P1", "Catti-brie of Mithral Hall": "P1P1",
}

func TestCostProgSubCounterAllLockstep(t *testing.T) {
	t.Parallel()
	for i, decks := range costProgSubCtrDecks {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			reg, set, gd := lockstepPool(t, "costprogsubctr", decks)
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for seed := uint64(1); seed <= 24; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"costprog-a", "costprog-b"}, gd, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				saw := false
				prev := map[state.ObjID]int32{}
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					g := ref.E.G
					cur := map[state.ObjID]int32{}
					for p := state.PlayerID(0); p < 2; p++ {
						for _, o := range g.Zone(state.ZBattlefield, p) {
							ob := g.Obj(o)
							if ob.Card == nil || len(ob.Card.Faces) == 0 {
								continue
							}
							kind, ok := costProgPile[ob.Card.Faces[0].Name]
							if !ok {
								continue
							}
							n := ob.Counter(kind)
							cur[o] = n
							if prev[o] > 0 && n == 0 {
								saw = true // a pile left the carrier: the cost removed it
							}
						}
					}
					prev = cur
					return nil
				}
				res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
				}
				if saw {
					seen++
				}
			}
			if seen == 0 {
				t.Fatal("no seed paid a SubCounter<All/KIND> cost with a counter on the carrier")
			}
			t.Logf("%d of 24 seeds paid a SubCounter<All/KIND> cost with a counter on the carrier", seen)
		})
	}
}
