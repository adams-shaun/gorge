package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The optional-move lockstep: a known-origin ChangeZone's
// Optional$ ask (gorge each object the move loop would move is
// offered to the chooser as a yes/no) on Mistbreath Elder's upkeep return
// and Intimidation Campaign's crime return, and an optional ChooseCard of up
// to two creature cards among Thunderous Debut's looked-at library cards.
// Shock aimed at the opponent is the crime. The frozen FDN and BLB pair
// pools exercise neither arm.

var koptmovesDecks = [2][]cardset.Entry{
	{
		{Name: "Thunderous Debut", Count: 4}, {Name: "Mistbreath Elder", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 2},
		{Name: "Giant Spider", Count: 2}, {Name: "Forest", Count: 20},
	},
	{
		{Name: "Intimidation Campaign", Count: 4}, {Name: "Shock", Count: 6},
		{Name: "Walking Corpse", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

// koptmovesStats counts what the policy saw gorge pose.
type koptmovesStats struct {
	asks    map[string]int // answers to a changezone_optional ask, by option kind
	debut   int            // Thunderous Debut resolutions that posed the pick
	picks   map[int]int    // by number of cards picked
	started map[*GorgeSide]bool
}

// koptmovesPolicy is the random policy that tallies the optional-move
// answers and the Debut pick counts from gorge's pending decision.
func koptmovesPolicy(seed uint64, st *koptmovesStats) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil {
			return a
		}
		switch d.ResumeKind {
		case "changezone_optional":
			if o, ok := c.G.GorgeOption(a); ok {
				st.asks[o.Kind]++
			}
		case "choice":
			ob := c.G.E.G.Obj(d.Source)
			if ob == nil || ob.Face() == nil || ob.Face().Name != "Thunderous Debut" {
				break
			}
			n := 0
			if c.G.fr != nil {
				n = len(c.G.fr.chosen)
			}
			if a.Kind() == ActDone {
				st.debut++
				st.picks[n]++
			} else if n+1 >= d.Max {
				st.debut++
				st.picks[n+1]++
			}
		}
		return a
	})
}

func runOptmoves(t *testing.T, seeds int, mutate bool) (games, divs int, st *koptmovesStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("optmoves")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugOptMoveAlways
	dirac.DebugOptMoveAlways = mutate
	defer func() { dirac.DebugOptMoveAlways = old }()
	st = &koptmovesStats{asks: map[string]int{}, picks: map[int]int{}}
	var decks [2][]*cards.Card
	for s, d := range koptmovesDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(koptmovesDecks[0], koptmovesDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, koptmovesNames, decks, 1)
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
		res := LockstepLed(m, seed, g, k, koptmovesPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && !mutate {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
	}
	return games, divs, st
}

var koptmovesNames = [2]string{"koptmoves-a", "koptmoves-b"}

func TestOptmovesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, st := runOptmoves(t, 256, false)
	t.Logf("%d games, %d divergences, optional-move asks %v, Thunderous Debut resolutions %d by picks %v", games, divs, st.asks, st.debut, st.picks)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if st.asks["yes"] == 0 || st.asks["no"] == 0 || st.asks["yes"]+st.asks["no"] < 30 {
		t.Errorf("optional-move asks %v: want both answers and at least 30", st.asks)
	}
	if st.debut < 15 || st.picks[0] == 0 || st.picks[1] == 0 || st.picks[2] == 0 {
		t.Errorf("Thunderous Debut resolutions %d by picks %v: want 15 and 0, 1 and 2 picks", st.debut, st.picks)
	}
}

// TestOptmovesMutation moves without the ask (the earlier gorge): gorge
// still asks, so the decision streams must differ. A green mutation (no
// divergence) means the lockstep never reaches an optional move.
func TestOptmovesMutation(t *testing.T) {
	games, divs, _ := runOptmoves(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation moved unasked in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
