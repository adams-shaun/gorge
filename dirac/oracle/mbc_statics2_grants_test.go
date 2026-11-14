package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCStatics2GrantsLowers plays the MBC statics-2 granted-static
// spellings kgen lowers in lockstep with gorge over the mbcstatics2grants
// fixture pool (dirac/cardset/pools/fixtures/mbcstatics2grants), one subtest
// per spelling:
//
//   - zulaport: Level up (CR 702.87a, the LEVEL counter) and a level band
//     gated on two counter compares (Card.Self+counters_GE1_LEVEL+
//     counters_LT3_LEVEL) whose LEVEL 3+ band grants a CantBlockBy static
//     (AddStaticAbility$ on the source);
//   - kabira: the same keyword and a LEVEL 5+ band granting an "other
//     creatures you control get +2/+2" Continuous static beside the 2-4
//     band's +1/+1 (counters_GE2_LEVEL+counters_LE4_LEVEL);
//   - tsagan: a max-speed grant of two statics, "SelfDT & WideFS" (deathtouch
//     on itself, first strike on the controller's other creatures), which
//     gorge grants in the value's order (parseGrantedStatics).
//
// Each subtest asserts the precondition its rule needs (a Zulaport at level
// 3 or more; a Kabira at level 5 or more with another creature beside it;
// Tsagan at max speed beside another creature) was reached in a played game,
// and that the unmutated kernel does not diverge. Its mutation subtest
// drops the grant's gate (the static's Cond2) from the table and must
// diverge.
func TestMBCStatics2GrantsLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range statics2Cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("lockstep", func(t *testing.T) {
				run := runStatics2Lockstep(t, c, 60, true, false)
				t.Logf("%d games, %d divergences, %d stalls; precondition reached in %d games", run.games, run.divs, run.stalls, run.hits)
				if run.hits == 0 {
					t.Errorf("%s: the precondition (%s) was never reached in %d games", c.name, c.want, run.games)
				}
			})
			t.Run("mutation", func(t *testing.T) {
				run := runStatics2Lockstep(t, c, 60, false, true)
				run.requireDivergence(t, c.name+": the grant's gate dropped from the kernel")
			})
		})
	}
}

type statics2Case struct {
	name    string
	decks   fixtureDecks
	watch   map[string]bool
	want    string
	reached func(g *GorgeSide) bool
	mutate  func(t *testing.T, tbl *kop.Table)
}

func statics2Entries(n ...any) []cardset.Entry {
	var out []cardset.Entry
	for i := 0; i < len(n); i += 2 {
		out = append(out, cardset.Entry{Name: n[i].(string), Count: n[i+1].(int)})
	}
	return out
}

// statics2Perm reports whether some battlefield permanent named name satisfies ok.
func statics2Perm(g *GorgeSide, name string, ok func(o *state.Object) bool) bool {
	gm := g.E.G
	for i := range gm.Objs {
		o := &gm.Objs[i]
		if o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == name && ok(o) {
			return true
		}
	}
	return false
}

// statics2Beside reports whether the controller of o controls another creature.
func statics2Beside(g *GorgeSide, o *state.Object) bool {
	gm := g.E.G
	for i := range gm.Objs {
		p := &gm.Objs[i]
		if p.ID != o.ID && p.Zone == state.ZBattlefield && p.Controller == o.Controller && objectHasType(p, "Creature") {
			return true
		}
	}
	return false
}

// statics2DropCond clears the gate of every static of the named card that
// pick selects.
func statics2DropCond(t *testing.T, tbl *kop.Table, name string, pick func(s *kop.Static) bool) {
	t.Helper()
	n := 0
	mutateStatic(t, tbl, name, func(a *kop.Ability) {
		if a.Kind == kop.AbStatic && pick(&a.Static) && a.Static.Cond2 != 0 {
			a.Static.Cond2 = 0
			n++
		}
	})
	if n == 0 {
		t.Fatalf("%s: no gated static to drop in the table", name)
	}
}

var statics2Cases = []statics2Case{
	{
		name: "zulaport",
		decks: fixtureDecks{
			statics2Entries("Zulaport Enforcer", 10, "Walking Corpse", 6, "Swamp", 24),
			statics2Entries("Gray Ogre", 10, "Raging Goblin", 8, "Mountain", 18, "Zulaport Enforcer", 4),
		},
		watch: map[string]bool{"Zulaport Enforcer": true},
		want:  "a Zulaport Enforcer holding three LEVEL counters",
		reached: func(g *GorgeSide) bool {
			return statics2Perm(g, "Zulaport Enforcer", func(o *state.Object) bool { return o.Counter("LEVEL") >= 3 })
		},
		mutate: func(t *testing.T, tbl *kop.Table) {
			statics2DropCond(t, tbl, "Zulaport Enforcer", func(s *kop.Static) bool { return s.Kind == kop.StatCantBlockBy })
		},
	},
	{
		name: "kabira",
		decks: fixtureDecks{
			statics2Entries("Kabira Vindicator", 10, "Savannah Lions", 8, "Plains", 22),
			statics2Entries("Gray Ogre", 10, "Raging Goblin", 8, "Mountain", 18, "Kabira Vindicator", 4),
		},
		watch: map[string]bool{"Kabira Vindicator": true},
		want:  "a Kabira Vindicator holding five LEVEL counters beside another creature",
		reached: func(g *GorgeSide) bool {
			return statics2Perm(g, "Kabira Vindicator", func(o *state.Object) bool {
				return o.Counter("LEVEL") >= 5 && statics2Beside(g, o)
			})
		},
		mutate: func(t *testing.T, tbl *kop.Table) {
			statics2DropCond(t, tbl, "Kabira Vindicator", func(s *kop.Static) bool { return s.Kind == kop.StatContinuous && s.A == 2 && s.B == 2 })
		},
	},
	{
		name: "tsagan",
		decks: fixtureDecks{
			statics2Entries("Tsagan, Raider Warlord", 8, "Raging Goblin", 8, "Savannah Lions", 4, "Mountain", 7, "Plains", 7, "Swamp", 6),
			statics2Entries("Gray Ogre", 4, "Raging Goblin", 4, "Mountain", 32),
		},
		watch: map[string]bool{"Tsagan, Raider Warlord": true, "Raging Goblin": true, "Savannah Lions": true},
		want:  "Tsagan at max speed beside another creature",
		reached: func(g *GorgeSide) bool {
			return statics2Perm(g, "Tsagan, Raider Warlord", func(o *state.Object) bool {
				return g.E.G.Players[o.Controller].Speed >= 4 && statics2Beside(g, o)
			})
		},
		mutate: func(t *testing.T, tbl *kop.Table) {
			statics2DropCond(t, tbl, "Tsagan, Raider Warlord", func(s *kop.Static) bool { return s.Kind == kop.StatContinuous && s.Kw != 0 })
		},
	},
}

type statics2Run struct {
	fixtureRun
	hits int
}

// statics2Policy plays randomly but casts a watched card and, for a Level
// up carrier, activates its level up the moment either is offered (seven
// times in eight), so the levelled bands are reached.
func statics2Policy(seed uint64, c statics2Case, hit *bool) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x73746132)) // "sta2"
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G != nil {
			if !*hit && c.reached(p.G) {
				*hit = true
			}
			if p.D.Kind == DPriority {
				for _, a := range p.D.Options {
					if k := a.Kind(); k != ActActivate && k != ActCast {
						continue
					}
					if c.watch[refCardName(p.G, a.Src())] && prng.IntN(8) != 0 {
						return a
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

func runStatics2Lockstep(t *testing.T, c statics2Case, seeds int, report, mutate bool) statics2Run {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcstatics2grants")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range c.decks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("%s deck %d: %d cards", c.name, s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		c.mutate(t, tbl)
	}
	run := statics2Run{fixtureRun: fixtureRun{casts: map[string]int{}}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"statics2-a", "statics2-b"}, decks, 1)
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
		hit := false
		res := LockstepLed(m, seed, g, k, statics2Policy(seed, c, &hit), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40})
		CloseSide(k)
		run.games++
		if hit {
			run.hits++
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
	return run
}
