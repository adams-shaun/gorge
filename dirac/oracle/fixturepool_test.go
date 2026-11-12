package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// fixtureRun is the result of playing a fixture pool's constructed decks in
// lockstep over a range of seeds.
type fixtureRun struct {
	games, divs, stalls int
	scripted            int            // gorge Stats.Scripted summed over the games
	casts               map[string]int // watched cards cast, by name
	first               string         // the first divergence, for the failure message
}

// fixtureDecks is one constructed matchup over a fixture pool: each of the
// two decks is a 40-card list of the fixture's cards.
type fixtureDecks [2][]cardset.Entry

// fixtureWatchPolicy plays randomly but takes a cast of a watched card the
// moment one is offered (three times in four), so the carriers of a fixture
// are played in most games and their casts are counted.
func fixtureWatchPolicy(seed uint64, watch map[string]bool, casts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x66697874)) // "fixt"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if n := refCardName(c.G, a.Src()); watch[n] && prng.IntN(4) != 0 {
					casts[n]++
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runFixtureLockstep plays seeds 1..seeds of the matchups over the fixture
// pool set, alternating the matchup and the option surface by seed. With
// report it fails the test on each divergence (up to three); without it, it
// stops at the first and returns. mutate, when set, rewrites each matchup's
// table before the games are played (the mutation tests' feature switch).
func runFixtureLockstep(t *testing.T, fixture string, matchups []fixtureDecks, watch map[string]bool, seeds int, report bool, mutate func(*kop.Table)) fixtureRun {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(fixture)
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(matchups))
	tbls := make([]*kop.Table, len(matchups))
	for mi, md := range matchups {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("%s matchup %d deck %d: %d cards", fixture, mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		tbls[mi] = tbl
	}
	run := fixtureRun{casts: map[string]int{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(matchups)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, watch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		run.scripted += g.Stats.Scripted
		CloseSide(k)
		run.games++
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
	return run
}

// requireDivergence is the mutation tests' check: the mutated kernel must
// diverge from gorge in some game.
func (r fixtureRun) requireDivergence(t *testing.T, what string) {
	t.Helper()
	if r.divs == 0 {
		t.Fatalf("%s in %d games with no divergence", what, r.games)
	}
	t.Logf("mutation diverged in %d of %d games: %s", r.divs, r.games, r.first)
}

// mutateCard rewrites one card's lowered programs in a matchup table, on
// copies so the set's own table (shared by later tests in the process) is not
// touched. fn edits each instruction of every ability of the named card.
func mutateCard(t *testing.T, tbl *kop.Table, name string, fn func(a *kop.Ability, in *kop.Instr)) {
	t.Helper()
	found := false
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		if cd.Name != name {
			continue
		}
		found = true
		cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			a.Prog = append([]kop.Instr(nil), a.Prog...)
			for x := range a.Prog {
				fn(a, &a.Prog[x])
			}
		}
	}
	if !found {
		t.Fatalf("%s is not in the table", name)
	}
}
