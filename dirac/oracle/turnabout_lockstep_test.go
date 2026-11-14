package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Turnabout (api:TapOrUntapAll): one tap-or-untap election, then every
// permanent of the chosen type the target player controls is tapped or
// untapped (gorge effTapOrUntapAll, kgen lower_tapuntap.go tapOrUntapAll,
// kernel act_taporuntapall.go), played from the turnabout fixture pool.

var turnaboutDecks = [2][]cardset.Entry{
	{
		{Name: "Turnabout", Count: 8}, {Name: "Ornithopter", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Island", Count: 13}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Ornithopter", Count: 4},
		{Name: "Forest", Count: 11}, {Name: "Mountain", Count: 11},
	},
}

// turnaboutStats counts gorge's Turnabout elections by the answer given.
type turnaboutStats struct {
	cast, tap, untap int
}

// turnaboutPolicy casts Turnabout whenever it can and answers its election
// at random, counting each answer; everything else is RandomPolicy's.
func turnaboutPolicy(seed uint64, st *turnaboutStats) Policy {
	inner := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7475726e61)) // "turna"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		g := c.G.E.G
		if c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if opt, ok := c.G.GorgeOption(a); ok && chName(g, opt.Obj) == "Turnabout" && prng.IntN(4) != 0 {
					st.cast++
					return a
				}
			}
			return inner.Pick(c)
		}
		if pd := c.G.Pending(); pd != nil && pd.ResumeKind == "taporuntapall" && len(c.D.Options) == 2 {
			a := c.D.Options[prng.IntN(2)]
			if opt, ok := c.G.GorgeOption(a); ok {
				switch opt.Kind {
				case "tap":
					st.tap++
				case "untap":
					st.untap++
				}
			}
			return a
		}
		return inner.Pick(c)
	})
}

// runTurnabout plays the matchup in both seatings, seeds 1..seeds each.
func runTurnabout(t *testing.T, seeds int) (games, divs, stalls int, st turnaboutStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("turnabout")
	if err != nil {
		t.Fatal(err)
	}
	sup := set.Supported()
	if i, ok := set.Lookup("Turnabout"); !ok || i >= len(sup) || !sup[i] {
		t.Fatalf("the turnabout pool does not play Turnabout: %s", set.Reason(i))
	}
	for swap := 0; swap < 2; swap++ {
		md := turnaboutDecks
		if swap == 1 {
			md = [2][]cardset.Entry{md[1], md[0]}
		}
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
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"turnabout-a", "turnabout-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, turnaboutPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

// TestTurnaboutLockstep: Turnabout plays in lockstep against gorge, with both
// answers to its one election given.
func TestTurnaboutLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runTurnabout(t, 16)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.tap < 3 || st.untap < 3 {
		t.Errorf("elections tap %d / untap %d (want >= 3 each)", st.tap, st.untap)
	}
}
