package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Evoke + Adamant lockstep (CR 702.74, CR 702.5): constructed decks over
// the generated evoke pool, so every evoked cast gorge offers (hybrid pips
// announced), the sacrifice trigger queued behind the Incarnation's own
// enters triggers, the Adamant intervening-if gates read as they trigger and
// again as they resolve (after the evoked permanent was sacrificed
// first, a paid gate still resolves on the cast-time spend) are all answered
// by the kernel.

var evokeDecks = [2][]cardset.Entry{
	{ // A: R/W/G
		{Name: "Catharsis", Count: 2}, {Name: "Vibrance", Count: 2}, {Name: "Wistfulness", Count: 1},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 4},
		{Name: "Wind-Scarred Crag", Count: 2}, {Name: "Rugged Highlands", Count: 1}, {Name: "Blossoming Sands", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Air Elemental", Count: 4},
	},
	{ // B: U/B/W/G
		{Name: "Deceit", Count: 2}, {Name: "Emptiness", Count: 2}, {Name: "Wistfulness", Count: 2},
		{Name: "Island", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Plains", Count: 3}, {Name: "Forest", Count: 2},
		{Name: "Dismal Backwater", Count: 2}, {Name: "Scoured Barrens", Count: 2}, {Name: "Thornwood Falls", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Air Elemental", Count: 3},
	},
}

// evokeProbe wraps a seeded random policy biased to cast the evoked and the
// hard cast of an Incarnation and, when an evoked permanent's pending
// triggers are ordered, to push its sacrifice trigger last (so it resolves
// first, ahead of the paid Adamant gates).
type evokeProbe struct {
	base Policy
	r    *rand.Rand
}

func (p *evokeProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G != nil && a.Kind() == ActCast {
		var evoked, hard []Canon
		for _, x := range c.D.Options {
			if o, ok := c.G.GorgeOption(x); ok && x.Kind() == ActCast {
				switch o.Mode {
				case "evoked":
					evoked = append(evoked, x)
				case "":
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && strings.Contains(strings.Join(ob.Face().Types, " "), "Incarnation") {
						hard = append(hard, x)
					}
				}
			}
		}
		switch {
		case len(evoked) > 0 && p.r.IntN(2) == 0:
			a = evoked[p.r.IntN(len(evoked))]
		case len(hard) > 0 && p.r.IntN(5) != 0:
			a = hard[p.r.IntN(len(hard))]
		}
	}
	if a.Kind() == ActOrder {
		// the sacrifice trigger is queued last (the highest occurrence of its
		// source): pick the others first, so it goes on the stack last
		hi := Canon(0)
		for _, x := range c.D.Options {
			if x.Kind() == ActOrder && (hi == 0 || x.Mode() > hi.Mode()) {
				hi = x
			}
		}
		if len(c.D.Options) > 1 && p.r.IntN(4) != 0 {
			var rest []Canon
			for _, x := range c.D.Options {
				if x != hi {
					rest = append(rest, x)
				}
			}
			a = rest[p.r.IntN(len(rest))]
		}
	}
	return a
}

// evokeSetup builds the matchup's corpus decks and table.
func evokeSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("evoke")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range evokeDecks {
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
	tbl, _, _, err := set.Matchup(evokeDecks[0], evokeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// evokeRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count, the kernel's tallies and the games with a
// sacrifice-first recheck that held. report logs the first divergence as a
// test error (the mutation runs expect one and stay quiet).
func evokeRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st dirac.EvokeStats, lkiGames int) {
	t.Helper()
	reg, set, tbl, decks := evokeSetup(t)
	dirac.DebugEvokeStats = &st
	defer func() { dirac.DebugEvokeStats = nil }()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"evoke-a", "evoke-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &evokeProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x65766b65))}
		before := st.SnapHolds
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if st.SnapHolds > before {
			lkiGames++
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestEvokeLockstep(t *testing.T) {
	div, stalls, st, lkiGames := evokeRun(t, 256, true)
	t.Logf("256 games, %+v, %d games with a held sacrifice-first recheck, %d divergences, %d stalls", st, lkiGames, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.Evoked, 40, "evoke casts"},
		{st.Hard, 40, "hard casts"},
		{st.GateTrue, 20, "Adamant gates true"},
		{st.GateFalse, 20, "Adamant gates false"},
		{lkiGames, 10, "games where a paid gate resolved after the evoked permanent left"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestEvokeMutation: skipping the evoke sacrifice trigger must make the
// lockstep diverge.
func TestEvokeMutation(t *testing.T) {
	dirac.DebugEvokeNoSac = true
	defer func() { dirac.DebugEvokeNoSac = false }()
	if diverged, _, _, _ := evokeRun(t, 64, false); !diverged {
		t.Fatal("DebugEvokeNoSac: no divergence in 64 seeds")
	}
}

// TestEvokeLKIMutation: an Adamant recheck that reads the live, wiped spend
// once the source left must make the lockstep diverge.
func TestEvokeLKIMutation(t *testing.T) {
	dirac.DebugAdamantLiveOnly = true
	defer func() { dirac.DebugAdamantLiveOnly = false }()
	if diverged, _, _, _ := evokeRun(t, 128, false); !diverged {
		t.Fatal("DebugAdamantLiveOnly: no divergence in 128 seeds")
	}
}
