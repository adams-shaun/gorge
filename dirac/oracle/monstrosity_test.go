package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Monstrosity lockstep: constructed matchups over the generated
// monstrosity pool. Fleecemane Lion and Domesticated Hydra read the
// IsMonstrous designation through a static keyword grant, and the other
// carriers trigger on BecomeMonstrous with destroy, mass-exile, sacrifice
// and emblem bodies. The frozen FDN and BLB pair pools exercise none of
// these arms.

var monstrosityDecks = [][2][]cardset.Entry{{
	{
		{Name: "Fleecemane Lion", Count: 4}, {Name: "Domesticated Hydra", Count: 4},
		{Name: "Arbor Colossus", Count: 4}, {Name: "Kalemne's Captain", Count: 4},
		{Name: "Protector of the Wastes", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		// One copy each of the two deathtouch carriers: the kernel keeps two
		// deathtouch marks (pfDeathtouched, pfDtMore) and gorge one counter per
		// event, so three or more deathtouch blockers on one attacker would
		// diverge on a generic cap, not on monstrosity.
		{Name: "Keepsake Gorgon", Count: 1}, {Name: "Hythonia the Cruel", Count: 1},
		{Name: "Alpha Deathclaw", Count: 4}, {Name: "Splitting Slime", Count: 4},
		{Name: "Ember Swallower", Count: 4},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 8},
	},
}}

// monstrosityCarriers names the cards whose casts and activations the test
// counts (each must be played at least once).
var monstrosityCarriers = []string{
	"Fleecemane Lion", "Domesticated Hydra", "Arbor Colossus", "Kalemne's Captain", "Protector of the Wastes",
	"Keepsake Gorgon", "Hythonia the Cruel", "Alpha Deathclaw", "Splitting Slime", "Ember Swallower",
}

func TestMonstrosityLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("monstrosity")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range monstrosityDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 1024; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"monstrosity-a", "monstrosity-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := eagerPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for _, n := range monstrosityCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}

// TestMonstrosityMutation flips the debug switch that makes the kernel read
// every permanent as not monstrous and requires the lockstep to diverge:
// the offer gate and the resolution guard disappear, so a second activation
// places counters and fires BecomeMonstrous again where gorge does nothing.
func TestMonstrosityMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("monstrosity")
	if err != nil {
		t.Fatal(err)
	}
	md := monstrosityDecks[0]
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugIgnoreMonstrosity
	dirac.DebugIgnoreMonstrosity = true
	defer func() { dirac.DebugIgnoreMonstrosity = old }()
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"monstrosity-a", "monstrosity-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			return
		}
	}
	t.Fatal("debug switch did not diverge in 32 seeds")
}
