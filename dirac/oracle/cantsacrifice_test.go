package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Quark, Humble Proprietor: "You can't sacrifice Treasure tokens", and a
// Treasure for every upkeep. gorge never offers the Treasure's mana ability,
// never plans with it, and leaves it out of every sacrifice pick: Targ
// Wrangler's end-step "sacrifice a nonland permanent" and Dahar Master's
// "sacrifice another nonland permanent" cost. The kernel reads the same
// StatCantSacrifice at each choke point (static_cantsacrifice.go).
var cantSacDecks = [2][]cardset.Entry{
	{{Name: "Quark, Humble Proprietor", Count: 10}, {Name: "Targ Wrangler", Count: 8}, {Name: "Dahar Master", Count: 6}, {Name: "Mountain", Count: 16}},
	{{Name: "Quark, Humble Proprietor", Count: 6}, {Name: "Dahar Master", Count: 8}, {Name: "Targ Wrangler", Count: 8}, {Name: "Mountain", Count: 18}},
}

// runCantSac plays the matchup over seeds 1..seeds, both surfaces
// alternating, and returns the games and divergences.
func runCantSac(t *testing.T, seeds int) (games, divs int) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	var cs [2][]*cards.Card
	for s, d := range cantSacDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(cantSacDecks[0], cantSacDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"cantsac-a", "cantsac-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !dirac.DebugCantSacOff {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs
}

func TestCantSacrificeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs := runCantSac(t, 200)
	t.Logf("%d games, %d divergent", games, divs)
}

// TestCantSacrificeMutation proves the matchup reaches the restriction: with
// the kernel ignoring it, a Treasure gorge keeps is offered or spent.
func TestCantSacrificeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugCantSacOff
	dirac.DebugCantSacOff = true
	defer func() { dirac.DebugCantSacOff = old }()
	games, divs := runCantSac(t, 20)
	t.Logf("%d games, %d divergent with the restriction off", games, divs)
	if divs < games/2 {
		t.Fatalf("only %d of %d games diverged with the restriction off: the matchup does not reach it", divs, games)
	}
}
