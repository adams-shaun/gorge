package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The shock-land as-enters pay-life election lockstep:
// "As CARDNAME enters, you may pay N life. If you don't, it enters tapped."
// gorge poses it through Resolve's generic unless gate (effects/unless.go
// poseUnlessAsk, ResumeKind "unless_pay") to the land's controller, offering
// pay only while life >= N (CR 119.4), on a land play and on a permanent put
// onto the battlefield mid-resolution (Coiling Oracle's dig).
var kshockDecks = [2][]cardset.Entry{
	{
		{Name: "Blood Crypt", Count: 3}, {Name: "Sacred Foundry", Count: 3},
		{Name: "Stomping Ground", Count: 3}, {Name: "Steam Vents", Count: 3},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Hallowed Fountain", Count: 2}, {Name: "Overgrown Tomb", Count: 2},
		{Name: "Temple Garden", Count: 2}, {Name: "Breeding Pool", Count: 2},
		{Name: "Godless Shrine", Count: 2}, {Name: "Watery Grave", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Opt", Count: 3},
		{Name: "Coiling Oracle", Count: 4},
		{Name: "Island", Count: 3}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 3},
	},
}

// kshockWoodElves is deck B with Wood Elves for Coiling Oracle: its ETB
// search puts a Forest card onto the battlefield, and a shock land found
// (Overgrown Tomb, Temple Garden, Breeding Pool, Stomping Ground are Forests)
// queues its pay-life election mid-search. gorge suspends the search at the
// election and shuffles the library after the answer.
func kshockWoodElves() []cardset.Entry {
	d := append([]cardset.Entry(nil), kshockDecks[1]...)
	for i := range d {
		if d[i].Name == "Coiling Oracle" {
			d[i].Name = "Wood Elves"
		}
	}
	return d
}

// kshockFarseek is deck B with Farseek for Coiling Oracle: a shock land it finds
// enters tapped whatever the election's answer, tapping after the answer.
func kshockFarseek() []cardset.Entry {
	d := append([]cardset.Entry(nil), kshockDecks[1]...)
	for i := range d {
		if d[i].Name == "Coiling Oracle" {
			d[i].Name = "Farseek"
		}
	}
	return d
}

// kshockTally counts the elections gorge posed and the answers taken.
type kshockTally struct {
	games, divs, stalls              int
	paid, declined, declineOnly, mid int
}

// kshockPolicy plays a land whenever it can and pays an election with
// probability 1/2 (seed+decision parity); everything else is random.
func kshockPolicy(seed uint64, tal *kshockTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if d := c.G.E.Pending(); d != nil && d.ResumeKind == "unless_pay" {
				pay, decline := -1, -1
				for i := range d.Options {
					switch d.Options[i].Mode {
					case "unless_pay":
						pay = i
					case "unless_decline":
						decline = i
					}
				}
				if len(c.G.E.G.Stack) > 0 {
					tal.mid++
				}
				if pay < 0 {
					tal.declineOnly++
				}
				want := decline
				if pay >= 0 && (seed+uint64(c.Seq))%2 == 0 {
					want = pay
				}
				if want < 0 {
					return rnd.Pick(c)
				}
				if want == pay {
					tal.paid++
				} else {
					tal.declined++
				}
				for _, a := range c.D.Options {
					if a.Kind() == ActMode && int(a.Mode()) == want {
						return a
					}
				}
				return rnd.Pick(c)
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

func TestShockLockstep(t *testing.T) {
	t.Parallel() // its subtests run isolated, so the parent need not hold the serial phase
	t.Run("coiling-oracle", func(t *testing.T) { kshockLockstep(t, kshockDecks, false) })
	t.Run("wood-elves", func(t *testing.T) {
		kshockLockstep(t, [2][]cardset.Entry{kshockDecks[0], kshockWoodElves()}, true)
	})
	t.Run("farseek", func(t *testing.T) {
		kshockLockstep(t, [2][]cardset.Entry{kshockDecks[0], kshockFarseek()}, true)
	})
}

func kshockLockstep(t *testing.T, kdecks [2][]cardset.Entry, wood bool) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("shock")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdecks {
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
	tbl, _, _, err := set.Matchup(kdecks[0], kdecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var tal kshockTally
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kshock-a", "kshock-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kshockPolicy(seed, &tal), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 60})
		CloseSide(k)
		tal.games++
		if res.Stall != "" {
			tal.stalls++
			t.Logf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
		if d := res.Div; d != nil {
			tal.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls; elections: %d paid, %d declined, %d decline-only, %d mid-resolution",
		tal.games, tal.divs, tal.stalls, tal.paid, tal.declined, tal.declineOnly, tal.mid)
	if tal.divs != 0 || tal.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", tal.divs, tal.stalls)
	}
	if wood {
		// the elections taken with the search on the stack
		if tal.mid < 10 {
			t.Errorf("only %d mid-search elections, want >= 10", tal.mid)
		}
		return
	}
	if tal.paid < 30 {
		t.Errorf("only %d paid elections, want >= 30", tal.paid)
	}
	if tal.declined < 30 {
		t.Errorf("only %d declines, want >= 30", tal.declined)
	}
	if tal.declineOnly < 5 {
		t.Errorf("only %d decline-only asks, want >= 5", tal.declineOnly)
	}
	if tal.mid < 5 {
		t.Errorf("only %d mid-resolution elections, want >= 5", tal.mid)
	}
}
