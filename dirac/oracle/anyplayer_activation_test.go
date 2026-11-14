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

// TestAnyPlayerActivationLowers plays the Activator$ Player ("Any player may
// activate this ability", CR 602.2) spellings kgen lowers past Feral Hydra's
// PutCounter in lockstep with gorge over the anyplayeract fixture pool
// (dirac/cardset/pools/fixtures/anyplayeract): Flailing Ogre's two Pumps,
// Ribbon Snake's Debuff, Warmonger's DamageAll and Quicksilver Wall's
// ChangeZone to its owner's hand, each carrying kop.Ability.AnyActivator and
// offered to the opponent of its controller by dirac/cast_anyactivator.go,
// paid from the activator's sources and put on the stack under the activator.
//
// Preconditions (each fails the test when unmet): every ability of the four
// cards carries AnyActivator in the matchup table (the lowering reverted
// leaves the cards refused), and the game logs show each card activated by a
// player who does not control it. The mutation subtest drops AnyActivator
// from the four cards and must diverge: gorge offers the opponent the
// activation, the kernel must too.
func TestAnyPlayerActivationLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		var st anyActStats
		run := runAnyActLockstep(t, 40, true, requireAnyActivator, &st)
		t.Logf("%d games, %d divergences, %d stalls; activations by the controller %v, by an opponent %v",
			run.games, run.divs, run.stalls, st.own, st.opp)
		for _, n := range anyActCards {
			if st.opp[n] == 0 {
				t.Errorf("no %s was activated by a player who does not control it in %d games", n, run.games)
			}
		}
	})
	t.Run("mutation", func(t *testing.T) {
		run := runAnyActLockstep(t, 40, false, func(t *testing.T, tbl *kop.Table) {
			requireAnyActivator(t, tbl)
			for _, n := range anyActCards {
				mutateCard(t, tbl, n, func(a *kop.Ability, _ *kop.Instr) { a.AnyActivator = false })
			}
		}, nil)
		run.requireDivergence(t, "the any-player activations dropped from the kernel")
	})
}

var anyActCards = []string{"Flailing Ogre", "Ribbon Snake", "Warmonger", "Quicksilver Wall"}

var anyActDecks = fixtureDecks{
	{{Name: "Flailing Ogre", Count: 8}, {Name: "Warmonger", Count: 6}, {Name: "Quicksilver Wall", Count: 4}, {Name: "Mountain", Count: 13}, {Name: "Island", Count: 9}},
	{{Name: "Ribbon Snake", Count: 8}, {Name: "Quicksilver Wall", Count: 6}, {Name: "Flailing Ogre", Count: 4}, {Name: "Island", Count: 13}, {Name: "Mountain", Count: 9}},
}

// requireAnyActivator fails unless every activated ability of the watched
// cards carries AnyActivator: the precondition the lockstep's offers rest on.
func requireAnyActivator(t *testing.T, tbl *kop.Table) {
	t.Helper()
	seen := map[string]bool{}
	for _, cd := range tbl.Cards {
		for _, n := range anyActCards {
			if cd.Name != n {
				continue
			}
			for _, a := range cd.Abilities {
				if a.Kind != kop.AbActivated {
					continue
				}
				if !a.AnyActivator {
					t.Fatalf("%s: an activated ability without AnyActivator (Activator$ Player is not lowered)", n)
				}
				seen[n] = true
			}
		}
	}
	for _, n := range anyActCards {
		if !seen[n] {
			t.Fatalf("%s: no activated ability in the table (Activator$ Player is not lowered)", n)
		}
	}
}

type anyActStats struct{ own, opp map[string]int }

// anyActPolicy plays randomly but activates a watched card's ability --
// either player's -- the moment one is offered (seven times in eight), so
// the opponents' activations happen in most games. With st it tallies, at
// each decision, the activations gorge pushed since the last one (the source
// is still where it was activated).
func anyActPolicy(seed uint64, st *anyActStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x616e7961)) // "anya"
	watch := map[string]bool{}
	for _, n := range anyActCards {
		watch[n] = true
	}
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && st != nil {
			seen = st.tally(c.G, seen)
		}
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if k := a.Kind(); k != ActActivate && k != ActCast {
					continue
				}
				if watch[refCardName(c.G, a.Src())] && prng.IntN(8) != 0 {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// tally counts, in g's log from event from on, the watched cards'
// activations by their controller and by another player, and returns the
// log's length.
func (st *anyActStats) tally(g *GorgeSide, from int) int {
	if st.own == nil {
		st.own, st.opp = map[string]int{}, map[string]int{}
	}
	evs := g.E.L.Events
	for _, e := range evs[min(from, len(evs)):] {
		if e.Kind != events.AbilityPush {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
			continue
		}
		n := o.Card.Faces[0].Name
		if e.Player == o.Controller {
			st.own[n]++
		} else {
			st.opp[n]++
		}
	}
	return len(evs)
}

// runAnyActLockstep plays seeds 1..seeds of the anyplayeract matchup in
// lockstep with gorge, alternating the option surface by seed. prepare checks
// (and, for the mutation, rewrites) the matchup table. With report it fails
// the test on each divergence or kernel stall (up to three divergences);
// without it, it stops at the first divergence.
func runAnyActLockstep(t *testing.T, seeds int, report bool, prepare func(*testing.T, *kop.Table), st *anyActStats) fixtureRun {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("anyplayeract")
	if err != nil {
		t.Fatal(err)
	}
	md := anyActDecks
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
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
	prepare(t, tbl)
	run := fixtureRun{casts: map[string]int{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"anyact-a", "anyact-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, anyActPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
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
