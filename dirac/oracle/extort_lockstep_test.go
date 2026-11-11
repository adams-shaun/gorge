package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Extort class (CR 702.101): every named card prints K:Extort, and Pontiff
// of Blight also grants it to the other creatures its controller controls
// (CR 702.101b: each instance triggers separately). Casting a spell fires the
// printed trigger and, with Pontiff out, one granted trigger per holder; each
// asks {W/B} and drains every opponent on a pay.
var kextortDeck = []cardset.Entry{
	{Name: "Blind Obedience", Count: 3}, {Name: "Crypt Ghast", Count: 3},
	{Name: "Life Insurance", Count: 3}, {Name: "Pontiff of Blight", Count: 3},
	{Name: "Tithe Drinker", Count: 3}, {Name: "Treasury Thrull", Count: 3},
	{Name: "Dark Ritual", Count: 4}, {Name: "Sign in Blood", Count: 4},
	{Name: "Swamp", Count: 9}, {Name: "Plains", Count: 5},
}

// extortCards are the class carriers the test must actually cast (not merely
// list in the pool): the Done-means gate is that each newly lowered card's
// decisions are compared, not that the fixture mentions it.
var extortCards = []string{"Blind Obedience", "Crypt Ghast", "Life Insurance", "Pontiff of Blight", "Tithe Drinker", "Treasury Thrull"}

// TestExtortLockstep plays the class in lockstep and asserts the election was
// actually reached: the Check counts the Extort ask on the gorge side, and the
// policy always takes the first option of a two-mode ask (pay), so a clean run
// covers the paid drain as well as the decline.
func TestExtortLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("extort")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := 0; s < 2; s++ {
		for _, e := range kextortDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards, want 40", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kextortDeck, kextortDeck)
	if err != nil {
		t.Fatal(err)
	}
	asks := 0
	cast := map[string]bool{}
	for seed := uint64(1); seed <= 200; seed++ {
		m, err := NewMatchup(reg, [2]string{"extort-a", "extort-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := extortPayPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
			Check: func(g *GorgeSide, _ Side) *Divergence {
				if d := g.Pending(); d != nil && d.ResumeKind == "extort" {
					asks++
				}
				return nil
			},
			Finish: func(g *GorgeSide) {
				if g.E == nil {
					return
				}
				c := CoverageOf(g, m)
				for _, n := range extortCards {
					if cc := c.Cards[n]; cc != nil && cc.Cast > 0 {
						cast[n] = true
					}
				}
			}})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
			continue
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
	}
	if asks == 0 {
		t.Fatal("precondition: no Extort decision was posed in 200 seeds: the pool never cast a spell with an Extort permanent")
	}
	for _, n := range extortCards {
		if !cast[n] {
			t.Errorf("%s: never cast in 200 seeds, so its Extort decisions were not compared", n)
		}
	}
	t.Logf("extort: %d Extort elections, cast %v", asks, cast)
}

// extortPayPolicy plays castFirstPolicy but takes the first option of a
// two-mode ask, which is the Extort pay option (gorge option 0). The policy
// never sees the gorge decision, so other two-mode choices also take mode 0:
// both sides are compared on that same canonical answer.
func extortPayPolicy(seed uint64) Policy {
	cast := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DModes && len(c.D.Options) == 2 &&
			c.D.Options[0].Kind() == ActMode && c.D.Options[1].Kind() == ActMode {
			return c.D.Options[0]
		}
		return cast.Pick(c)
	})
}

// TestExtortDeclineLockstep is the decline side: a policy that always takes
// the second option of a two-mode ask must agree too (no drain, no payment).
func TestExtortDeclineLockstep(t *testing.T) {
	t.Parallel()
	// The decline arm is the same comparison as the pay arm; one focused
	// policy run of each keeps a drain bug from hiding behind a policy that
	// only ever pays. Reuse the pay test's setup with the opposite answer.
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("extort")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := 0; s < 2; s++ {
		for _, e := range kextortDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kextortDeck, kextortDeck)
	if err != nil {
		t.Fatal(err)
	}
	asks := 0
	for seed := uint64(1); seed <= 40; seed++ {
		m, err := NewMatchup(reg, [2]string{"extort-a", "extort-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := extortDeclinePolicy(seed)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
			Check: func(g *GorgeSide, _ Side) *Divergence {
				if d := g.Pending(); d != nil && d.ResumeKind == "extort" {
					asks++
				}
				return nil
			}})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
			continue
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if asks == 0 {
		t.Fatal("precondition: no Extort decision was posed in 40 seeds")
	}
}

func extortDeclinePolicy(seed uint64) Policy {
	cast := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DModes && len(c.D.Options) == 2 &&
			c.D.Options[0].Kind() == ActMode && c.D.Options[1].Kind() == ActMode {
			return c.D.Options[1]
		}
		return cast.Pick(c)
	})
}
