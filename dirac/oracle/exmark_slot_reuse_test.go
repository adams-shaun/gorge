package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// kexmSlotDecks: Combustion Technique's "exile it instead if it would die this
// turn" (DealDamage ReplaceDyingDefined$ Targeted) beside Burning-Tree Emissary
// (a 2/2 that is cast, dies and is recast in one turn, so its battlefield slot
// is reused), with Iroh, Grand Lotus and the cheap spells that feed the lockstep.
var kexmSlotDecks = [2][]cardset.Entry{
	{
		{Name: "Iroh, Grand Lotus", Count: 8}, {Name: "Opt", Count: 4}, {Name: "Think Twice", Count: 3},
		{Name: "Combustion Technique", Count: 4}, {Name: "Boomerang Basics", Count: 3}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Iroh, Grand Lotus", Count: 8}, {Name: "Opt", Count: 4}, {Name: "Think Twice", Count: 3},
		{Name: "Combustion Technique", Count: 4}, {Name: "Boomerang Basics", Count: 3}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// TestExileMarkSlotReuseLockstep: a creature a Combustion Technique exiled
// leaves its battlefield slot, a new creature takes the slot the same turn, and
// the next Combustion Technique on it exiles it at once, as gorge does: the dead
// creature's exile-instead record does not compete with the new one (CR 400.7,
// 614.1a), so the kernel parks no order ask gorge never makes.
func TestExileMarkSlotReuseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	var names [2][]string
	var decks [2][]*cards.Card
	for s, d := range kexmSlotDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(kexmSlotDecks[0], kexmSlotDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	exiled := 0
	// seeds that diverged before the fix, all on the full surface (odd seeds)
	for _, seed := range []uint64{35, 83, 177, 281} {
		cfg := Config{GorgeAutopaySurface: false}
		m, err := NewMatchup(reg, [2]string{"kexm-a", "kexm-b"}, decks, 1)
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
		var st kfbGrantStats
		res := LockstepLed(m, seed, g, k, kfbGrantPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 60})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.E.G.Zone(state.ZExile, p) {
				if f := g.E.G.Obj(id).Face(); f != nil && f.IsCreature() {
					exiled++
				}
			}
		}
	}
	if exiled < 4 {
		t.Errorf("only %d creatures exiled over the four games: the Combustion Technique kills this test needs did not happen", exiled)
	}
}
