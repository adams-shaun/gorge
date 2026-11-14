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

// TestMBCApiParams8Lowers plays the MBC api-params-8 spelling kgen lowers,
// ActivationLimit$ N >= 2 on an activated ability ("activate no more than
// twice each turn"), in lockstep with gorge over the mbcactlimit fixture pool
// (dirac/cardset/pools/fixtures/mbcactlimit): Fire-Belly Changeling (2),
// Sewer Rats (3) and Vampire Bats (2). The policy activates a watched card's
// ability whenever one is offered, so a source runs into its limit with mana
// to spare and gorge withholds the next activation (legal_loyalty.go
// activationLimitBlocked); the kernel must withhold it too (kop.ExActivations
// N 1, lower_activationlimitn.go) or the option lists diverge.
//
// Precondition: every card reaches its limit in some turn of gorge's log
// (and never passes it). The mutation subtest drops the lowered gate from
// the table and must diverge. The other api-params-8 spellings stay refused
// (NonBasicSpell$ is Cleave's text-changed SP line, Fumble's Attach Choices$
// rides a refused GainControl Defined$): see the report.
func TestMBCApiParams8Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("ActivationLimit", func(t *testing.T) {
		run, hits := runActLimitLockstep(t, 24, true, false)
		t.Logf("%d games, %d divergences, %d stalls, limit reached %v", run.games, run.divs, run.stalls, hits)
		for name := range mbcActLimitWatch {
			if hits[name] < 3 {
				t.Errorf("%s reached its ActivationLimit$ in %d turns, want at least 3", name, hits[name])
			}
		}
	})
	t.Run("mutation", func(t *testing.T) {
		run, _ := runActLimitLockstep(t, 24, false, true)
		run.requireDivergence(t, "an ActivationLimit$ N ability offered past its limit")
	})
}

// mbcActLimitWatch is each watched card's printed ActivationLimit$.
var mbcActLimitWatch = map[string]int{"Fire-Belly Changeling": 2, "Sewer Rats": 3, "Vampire Bats": 2}

var mbcActLimitDecks = []fixtureDecks{{
	{{Name: "Fire-Belly Changeling", Count: 14}, {Name: "Mountain", Count: 26}},
	{{Name: "Sewer Rats", Count: 8}, {Name: "Vampire Bats", Count: 6}, {Name: "Swamp", Count: 26}},
}}

// actLimitPolicy plays randomly but takes an activation or a cast of a
// watched card the moment one is offered (seven times in eight).
func actLimitPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x616c696d)) // "alim"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if k := a.Kind(); k != ActActivate && k != ActCast {
					continue
				}
				if _, ok := mbcActLimitWatch[refCardName(c.G, a.Src())]; ok && prng.IntN(8) != 0 {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runActLimitLockstep plays seeds 1..seeds of the mbcactlimit matchup, gorge
// leading, and counts per watched card the turns in which one of its objects
// was activated exactly its limit times (gorge's AbilityPush events). With
// mutate, every watched card's activation condition is dropped from the
// table first.
func runActLimitLockstep(t *testing.T, seeds int, report, mutate bool) (fixtureRun, map[string]int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcactlimit")
	if err != nil {
		t.Fatal(err)
	}
	md := mbcActLimitDecks[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for name := range mbcActLimitWatch {
		gated := false
		for _, cd := range tbl.Cards {
			if cd.Name != name {
				continue
			}
			for _, a := range cd.Abilities {
				gated = gated || a.Kind == kop.AbActivated && a.Trig.Cond != 0
			}
		}
		if !gated {
			t.Fatalf("%s: no activation condition in the table (the ActivationLimit$ gate is not lowered)", name)
		}
		if mutate {
			mutateCard(t, tbl, name, func(a *kop.Ability, _ *kop.Instr) {
				if a.Kind == kop.AbActivated {
					a.Trig.Cond = 0
				}
			})
		}
	}
	run := fixtureRun{casts: map[string]int{}}
	hits := map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"actlimit-a", "actlimit-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, actLimitPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		actLimitHits(t, g, hits)
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
	return run, hits
}

// actLimitHits adds, per watched card, the turns of g's log in which one of
// its objects was activated exactly its ActivationLimit$ times, and fails
// on a turn past it (gorge's own gate).
func actLimitHits(t *testing.T, g *GorgeSide, hits map[string]int) {
	t.Helper()
	type key struct {
		turn int
		obj  uint32
	}
	turn := 0
	n := map[key]int{}
	names := map[key]string{}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			turn++
		case events.AbilityPush:
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
				continue
			}
			name := o.Card.Faces[0].Name
			if _, ok := mbcActLimitWatch[name]; !ok {
				continue
			}
			kk := key{turn, uint32(e.Obj)}
			n[kk]++
			names[kk] = name
		}
	}
	for kk, c := range n {
		lim := mbcActLimitWatch[names[kk]]
		if c > lim {
			t.Errorf("%s activated %d times in turn %d, past its limit %d", names[kk], c, kk.turn, lim)
		}
		if c == lim {
			hits[names[kk]]++
		}
	}
}
