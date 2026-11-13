package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCCosts9Lowers plays the MBC costs-9 spellings kgen lowers, in
// lockstep with gorge over the mbccosts9 fixture pool
// (dirac/cardset/pools/fixtures/mbccosts9):
//
//   - morph cost "PayLife<N>": Zombie Cutthroat's "Morph--Pay 5 life"
//     (CardDef.MorphLife, dirac/face_morphlife.go). The policy casts it face
//     down and turns it up whenever gorge offers the special action, so the
//     life payment settles and gorge withholds the action below 5 life.
//
// Precondition: the watched card's watched event happens in gorge's log
// (counted below). Each mutation subtest drops its lowering from the table
// and must diverge. The other two costs-9 spellings (Thornscape
// Battlemage's two-part kicker, Wash Away's cleave Cost$) stay refused: see
// the report.
func TestMBCCosts9Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("MorphLife", func(t *testing.T) {
		run, ups := runCosts9Lockstep(t, 24, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, Zombie Cutthroat turned up %d times", run.games, run.divs, run.stalls, ups)
		if ups < 5 {
			t.Errorf("Zombie Cutthroat turned face up %d times, want at least 5", ups)
		}
	})
	t.Run("MorphLife/mutation", func(t *testing.T) {
		run, _ := runCosts9Lockstep(t, 24, false, func(t *testing.T, tbl *kop.Table) {
			n := 0
			for i := range tbl.Cards {
				if cd := &tbl.Cards[i]; cd.Name == "Zombie Cutthroat" {
					if cd.MorphLife != 5 || cd.MorphFamily != kop.MorphMorph {
						t.Fatalf("Zombie Cutthroat: MorphLife %d family %d, want 5 and Morph", cd.MorphLife, cd.MorphFamily)
					}
					cd.MorphLife = 0
					n++
				}
			}
			if n == 0 {
				t.Fatal("Zombie Cutthroat not in the table")
			}
		})
		run.requireDivergence(t, "a Morph--Pay 5 life turned up for free")
	})
}

// costs9Watch is the watched cards; the policy favours their casts and
// special actions.
var costs9Watch = map[string]bool{"Zombie Cutthroat": true, "Priest of Gix": true}

var costs9Decks = fixtureDecks{
	{{Name: "Zombie Cutthroat", Count: 12}, {Name: "Priest of Gix", Count: 8}, {Name: "Mox Jet", Count: 4}, {Name: "Swamp", Count: 16}},
	{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}},
}

// costs9Policy plays randomly but, seven times in eight, takes the first
// offered turn-up, face-down cast or cast of a watched card, in that order
// of preference: Priest of Gix floats the {B}{B}{B} a face-down Zombie
// Cutthroat is cast from (gorge prices that cast from the floating pool).
func costs9Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x636f3973)) // "co9s"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && prng.IntN(8) != 0 {
			for _, k := range [...]CanonKind{ActSpecial, ActCastFace, ActCast} {
				for _, a := range c.D.Options {
					if a.Kind() == k && costs9Watch[refCardName(c.G, a.Src())] {
						return a
					}
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runCosts9Lockstep plays seeds 1..seeds of the mbccosts9 matchup, gorge
// leading, and counts the TurnFaceUp events in gorge's logs (Zombie
// Cutthroat is the pool's one face-down card). mutate, when set, rewrites
// the table first.
func runCosts9Lockstep(t *testing.T, seeds int, report bool, mutate func(*testing.T, *kop.Table)) (fixtureRun, int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbccosts9")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range costs9Decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(costs9Decks[0], costs9Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(t, tbl)
	}
	run := fixtureRun{casts: map[string]int{}}
	ups := 0
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"costs9-a", "costs9-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, costs9Policy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		for _, e := range g.E.L.Events {
			if e.Kind == events.TurnFaceUp {
				ups++ // the pool's one face-down card
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, ups
}
