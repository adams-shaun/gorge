package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: CopyPermanent and Animate over Defined$
// Valid <filter> (Multiversal Incursion: a copy of each nontoken creature
// you control, except it isn't legendary; Sita Varma, Masked Racer: you may
// have each other creature you control become Sita's power until end of
// turn), over mana creatures and vanilla creatures. Deck B as briefed held
// 34 cards: three Forests and three Plains fill it to 40.

var kdefvalidDecks = [2][]cardset.Entry{
	{
		kdE("Multiversal Incursion", 4), kdE("Sita Varma, Masked Racer", 4), kdE("Hydroid Krasis", 4), kdE("Llanowar Elves", 4),
		kdE("Grizzly Bears", 4), kdE("Forest", 12), kdE("Island", 8),
	},
	{
		kdE("Savannah Lions", 4), kdE("Grizzly Bears", 4), kdE("Giant Growth", 4), kdE("Air Elemental", 2),
		kdE("Divination", 2), kdE("Sol Ring", 1), kdE("Forest", 11), kdE("Island", 4), kdE("Plains", 8),
	},
}

// kdvTally is one run's observed branches.
type kdvTally struct {
	multi         uint32 // Incursion resolutions that minted >= 2 copies
	asks, yes, no int    // Sita's Animate asks and their answers
}

// kdefvalidRun plays seeds 1..n; with report each divergence is an error,
// without it the first divergence returns diverged.
func kdefvalidRun(t *testing.T, n uint64, report bool) (tl kdvTally, diverged bool) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("defvalid")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdefvalidDecks {
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
	tbl, _, _, err := set.Matchup(kdefvalidDecks[0], kdefvalidDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugDefValidMulti = &tl.multi
	defer func() { dirac.DebugDefValidMulti = nil }()
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdefvalid-a", "kdefvalid-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := Policy(HeuristicPolicy(seed))
		if seed%3 == 0 {
			base = RandomPolicy(seed)
		}
		xcode, _ := chooseCode("x")
		probe := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				// Hydroid Krasis's X ask: its smallest X leaves the planner's
				// floated mana in the pool, the only way Sita's {X}{G}{G}{U}
				// is ever offered (gorge prices no {X} activation; paylifex_test.go)
				if c.D.Kind == DChoose && len(c.D.Options) > 0 && c.D.Options[0].Kind() == ActChoose+CanonKind(xcode) {
					if pd := c.G.Pending(); pd != nil {
						if ob := c.G.E.G.Obj(pd.Source); ob != nil && ob.Face() != nil && ob.Face().Name == "Hydroid Krasis" {
							return c.D.Options[0]
						}
					}
				}
				if d := c.G.Pending(); d != nil && d.ResumeKind == "animate_optional" {
					// Sita's election: alternate the answer by seed and
					// decision number, so both branches are exercised
					want := "yes"
					if (seed+uint64(c.Seq))%2 == 1 {
						want = "no"
					}
					for _, a := range c.D.Options {
						if o, ok := c.G.GorgeOption(a); ok && o.Kind == want {
							tl.asks++
							if want == "yes" {
								tl.yes++
							} else {
								tl.no++
							}
							return a
						}
					}
				}
				// activate Sita's exhaust ability whenever it is offered (a
				// heuristic main phase seldom spends {X}{G}{G}{U} on it)
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					if o, ok := c.G.GorgeOption(a); ok {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Sita Varma, Masked Racer" {
							return a
						}
					}
				}
			}
			return base.Pick(c)
		})
		res := Lockstep(m, seed, k, probe, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && report {
			t.Errorf("seed %d: stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			diverged = true
			if !report {
				t.Logf("mutant diverged at seed %d: %s", seed, d)
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

// TestDefvalidLockstep: 0 divergences, 0 stalls, and the branch minima.
func TestDefvalidLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	tl, _ := kdefvalidRun(t, 256, true)
	t.Logf("incursion multi-copy resolutions %d, sita asks %d (yes %d, no %d)", tl.multi, tl.asks, tl.yes, tl.no)
	if tl.multi < 10 {
		t.Errorf("incursion multi-copy resolutions %d, want >= 10", tl.multi)
	}
	if tl.asks < 20 || tl.yes < 5 || tl.no < 5 {
		t.Errorf("sita asks %d (yes %d, no %d), want >= 20 with >= 5 of each", tl.asks, tl.yes, tl.no)
	}
}

// TestDefvalidMutation: copyAllEach minting for the first permanent only
// must diverge.
func TestDefvalidMutation(t *testing.T) {
	dirac.DebugDefValidFirstOnly = true
	defer func() { dirac.DebugDefValidFirstOnly = false }()
	if _, div := kdefvalidRun(t, 128, false); !div {
		t.Fatal("first-only copyAllEach: no divergence in 128 seeds")
	}
}

// TestDefvalidAnimateMutation: a kernel that never poses MayGuardAnimateAll's
// ask must diverge.
func TestDefvalidAnimateMutation(t *testing.T) {
	dirac.DebugDefValidNoAnimateAsk = true
	defer func() { dirac.DebugDefValidNoAnimateAsk = false }()
	if _, div := kdefvalidRun(t, 128, false); !div {
		t.Fatal("no-animate-ask: no divergence in 128 seeds")
	}
}
