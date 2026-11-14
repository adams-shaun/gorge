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

// TestMBCCounters2Lowers plays the MBC counters-2 spellings kgen lowers in
// lockstep with gorge over the mbccounters2 fixture pool
// (dirac/cardset/pools/fixtures/mbccounters2):
//
//   - counter kind "SCREAM" (Endless Scream: it enters with X scream counters
//     and its enchanted creature gets +1/+0 for each), kop.CtrScream;
//   - PutCounter Activator$ Player (Feral Hydra: "any player may activate
//     this ability"), kop.Ability.AnyActivator, offered to the opponent of
//     the Hydra's controller by dirac/cast_anyactivator.go.
//
// The Cytoplast Root-Kin spelling (MoveCounter with no Source$ and a Defined$
// destination) and Consul's Lieutenant's Renown$ have no playable carrier
// here: both cards also need keywords kgen refuses (Graft, Renown), so the
// MoveCounter shape is pinned by dirac/kgen's TestMoveCounterTargetOrigin.
//
// Preconditions (each fails the test when unmet): the game log shows a SCREAM
// counter put on an Endless Scream, and a Feral Hydra activated by a player
// who does not control it. The mutation subtest drops AnyActivator from the
// table and must diverge: gorge offers the opponent the activation, the
// kernel must too.
func TestMBCCounters2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runCounters2Lockstep(t, 40, true, false)
		t.Logf("%d games, %d divergences, %d stalls; scream counters %d, Hydra activations by the controller %d, by an opponent %d",
			run.games, run.divs, run.stalls, st.scream, st.ownAct, st.oppAct)
		if st.scream == 0 {
			t.Errorf("no SCREAM counter was put on an Endless Scream in %d games", run.games)
		}
		if st.oppAct == 0 {
			t.Errorf("no Feral Hydra was activated by a player who does not control it in %d games", run.games)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		run, _ := runCounters2Lockstep(t, 40, false, true)
		run.requireDivergence(t, "Feral Hydra's any-player activation dropped from the kernel")
	})
}

var mbcCounters2Decks = fixtureDecks{
	{{Name: "Endless Scream", Count: 8}, {Name: "Feral Hydra", Count: 6}, {Name: "Swamp", Count: 13}, {Name: "Forest", Count: 13}},
	{{Name: "Feral Hydra", Count: 10}, {Name: "Endless Scream", Count: 4}, {Name: "Forest", Count: 13}, {Name: "Plains", Count: 13}},
}

type counters2Stats struct{ scream, ownAct, oppAct int }

// counters2Policy plays randomly but casts a watched card and activates a
// Feral Hydra (either player's) the moment one is offered (seven times in
// eight).
func counters2Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x63747232)) // "ctr2"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if k := a.Kind(); k != ActActivate && k != ActCast {
					continue
				}
				if n := refCardName(c.G, a.Src()); (n == "Feral Hydra" || n == "Endless Scream") && prng.IntN(8) != 0 {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runCounters2Lockstep(t *testing.T, seeds int, report, mutate bool) (fixtureRun, counters2Stats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbccounters2")
	if err != nil {
		t.Fatal(err)
	}
	md := mbcCounters2Decks
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
	any := false
	for _, cd := range tbl.Cards {
		if cd.Name != "Feral Hydra" {
			continue
		}
		for _, a := range cd.Abilities {
			any = any || a.Kind == kop.AbActivated && a.AnyActivator
		}
	}
	if !any {
		t.Fatal("Feral Hydra: no AnyActivator ability in the table (Activator$ Player is not lowered)")
	}
	if mutate {
		mutateCard(t, tbl, "Feral Hydra", func(a *kop.Ability, _ *kop.Instr) { a.AnyActivator = false })
	}
	run := fixtureRun{casts: map[string]int{}}
	var st counters2Stats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"counters2-a", "counters2-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, counters2Policy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		counters2Tally(g, &st)
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
	return run, st
}

// counters2Tally counts, in g's log, the SCREAM counters put on an Endless
// Scream and the Feral Hydra activations by its controller and by another
// player.
func counters2Tally(g *GorgeSide, st *counters2Stats) {
	name := func(e events.Event) string {
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
			return ""
		}
		return o.Card.Faces[0].Name
	}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.CounterChange:
			if e.Counter == "SCREAM" && e.Amount > 0 && name(e) == "Endless Scream" {
				st.scream++
			}
		case events.AbilityPush:
			if name(e) != "Feral Hydra" {
				continue
			}
			if e.Player == g.E.G.Obj(e.Obj).Controller {
				st.ownAct++
			} else {
				st.oppAct++
			}
		}
	}
}
