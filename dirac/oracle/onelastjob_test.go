package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// gorge (51ed6bce3) makes a ChangeZone whose AttachedTo$
// is a card filter ASK the controller which legal bearer the returned card
// attaches to (CR 303.4f; changeZoneAttachChoose). One Last Job's third
// Spree mode ("attached to a creature you control", Creature.YouCtrl) is the
// carrier. The pool pits it against Auras, an Equipment and Disenchant (to
// put them in a graveyard) and creatures to bear them; the policy casts One
// Last Job aggressively so the bearer ask runs with two or more creatures.

var oneLastJobDecks = [][2][]cardset.Entry{{
	{
		{Name: "One Last Job", Count: 5}, {Name: "Pacifism", Count: 3}, {Name: "Bonesplitter", Count: 3},
		{Name: "Disenchant", Count: 2}, {Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "One Last Job", Count: 5}, {Name: "Pacifism", Count: 3}, {Name: "Bonesplitter", Count: 3},
		{Name: "Disenchant", Count: 2}, {Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6},
	},
}, {
	{
		{Name: "One Last Job", Count: 5}, {Name: "Pacifism", Count: 3}, {Name: "Bonesplitter", Count: 3},
		{Name: "Disenchant", Count: 2}, {Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Disenchant", Count: 3}, {Name: "Pacifism", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 7},
	},
}}

type oneLastJobStats struct {
	casts int // the policy cast One Last Job
	asks  int // a changezone_attach bearer ask with two or more options was posed
}

// oneLastJobPolicy casts One Last Job aggressively (three times in four from
// hand when it is offered) and counts the bearer asks. The rest is the
// seeded random policy.
func oneLastJobPolicy(seed uint64, st *oneLastJobStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b666f6c3430)) // "kfol40"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		if d := c.G.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "changezone_attach" && len(d.Options) >= 2 {
			st.asks++
			return rnd.Pick(c)
		}
		if c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if o, ok := c.G.GorgeOption(a); ok && chName(g, o.Obj) == "One Last Job" {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 && prng.IntN(4) != 0 {
			st.casts++
			return buf[prng.IntN(len(buf))]
		}
		return rnd.Pick(c)
	})
}

// oneLastJobRun plays seeds 1..n both seatings of every matchup.
func oneLastJobRun(t *testing.T, n uint64, st *oneLastJobStats, stopAtDiv, quiet bool) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("onelastjob")
	if err != nil {
		t.Fatal(err)
	}
	divs := 0
	for mi, md0 := range oneLastJobDecks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
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
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= n; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"deck-a", "deck-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, oneLastJobPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					divs++
					if quiet {
						t.Logf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v",
							mi, swap, seed, d.At, d.What, d.Field, d.Diff)
					} else {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
							mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
					if stopAtDiv {
						return divs
					}
				}
			}
		}
	}
	return divs
}

func TestOneLastJobLockstep(t *testing.T) {
	var st oneLastJobStats
	divs := oneLastJobRun(t, 64, &st, false, false)
	t.Logf("256 games, %d divergences, %+v", divs, st)
	if st.casts < 8 || st.asks < 2 {
		t.Errorf("coverage: casts %d, bearer asks %d; want at least 8 and 2", st.casts, st.asks)
	}
	if divs != 0 {
		t.Fatalf("%d divergences, want 0", divs)
	}
}

// TestOneLastJobBearerMutation attaches the first candidate bearer without asking
// (gorge's earlier pick); the pool must diverge. Not parallel: it flips a
// package variable.
func TestOneLastJobBearerMutation(t *testing.T) {
	dirac.DebugBearerFirst = true
	defer func() { dirac.DebugBearerFirst = false }()
	divs := oneLastJobRun(t, 64, &oneLastJobStats{}, true, true)
	if divs == 0 {
		t.Fatal("no seed diverged with the first bearer auto-picked")
	}
	t.Logf("mutation diverged (stopped at the first)")
}
