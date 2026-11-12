package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The copy-token end-step lockstep: the two flavours of a
// token copy that goes away at the end step have different delayed-trigger
// sources. Electroduplicate and Chandra, Flameshaper (AtEOTTrig$ Sacrifice)
// give the token its own trigger, so the sacrifice trigger stays sourced on
// the token; Stormsplitter (AtEOT$ Exile) is a one-shot rider sourced on the
// creating permanent (CR 603.7d-e; gorge).

var kcopyateotPools = []struct {
	set   string
	decks [2][]cardset.Entry
}{{
	// AtEOTTrig$ Sacrifice: the token carries the trigger
	set: "fdn",
	decks: [2][]cardset.Entry{{
		{Name: "Electroduplicate", Count: 8}, {Name: "Chandra, Flameshaper", Count: 3},
		{Name: "Frenzied Goblin", Count: 4}, {Name: "Courageous Goblin", Count: 4}, {Name: "Goblin Boarders", Count: 4},
		{Name: "Burst Lightning", Count: 3}, {Name: "Mountain", Count: 14},
	}, {
		{Name: "Electroduplicate", Count: 4}, {Name: "Chandra, Flameshaper", Count: 2},
		{Name: "Frenzied Goblin", Count: 5}, {Name: "Courageous Goblin", Count: 5}, {Name: "Goblin Boarders", Count: 5},
		{Name: "Burst Lightning", Count: 3}, {Name: "Mountain", Count: 16},
	}},
}, {
	// AtEOT$ Exile: the creating Stormsplitter is the source
	set: "blb",
	decks: [2][]cardset.Entry{{
		{Name: "Stormsplitter", Count: 8}, {Name: "High Stride", Count: 8},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
	}, {
		{Name: "Stormsplitter", Count: 6}, {Name: "High Stride", Count: 8}, {Name: "Coruscation Mage", Count: 4},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
	}},
}}

func TestCopyAtEndOfTurnLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	var tokens [2]int // copy tokens seen on gorge's battlefield, per pool
	for pi, pool := range kcopyateotPools {
		set, err := cardset.RawByName(pool.set)
		if err != nil {
			t.Fatal(err)
		}
		var decks [2][]*cards.Card
		for s, d := range pool.decks {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("pool %d deck %d: %d cards", pi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(pool.decks[0], pool.decks[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 96; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcopyateot-a", "kcopyateot-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[state.ObjID]bool{}
			pol := castActivatePolicy(seed)
			base := pol
			pol = PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					g := c.G.E.G
					for p := state.PlayerID(0); p < 2; p++ {
						for _, id := range g.Zone(state.ZBattlefield, p) {
							if o := g.Obj(id); o != nil && o.IsToken && !seen[id] {
								seen[id] = true
								tokens[pi]++
							}
						}
					}
				}
				return base.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				t.Errorf("pool %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", pi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games; copy tokens seen: %d (Electroduplicate/Chandra), %d (Stormsplitter)", games, tokens[0], tokens[1])
	if tokens[0] < 5 || tokens[1] < 5 {
		t.Errorf("thin coverage: tokens %v (want 5 each)", tokens)
	}
}
