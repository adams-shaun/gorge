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

// The Harmonize lockstep (CR 702.180): constructed U/R and G/U/R decks over
// the generated harmonize pool, so every graveyard harmonize cast gorge
// offers, and the announcement of the one creature tapped for its power
// (pay.ConvokeAsk Kind "harmonize"), is answered by the kernel's flashback
// cast and convoke stage.

var harmonizeDecks = [2][]cardset.Entry{
	{ // A: U/R
		{Name: "Channeled Dragonfire", Count: 2}, {Name: "Glacial Dragonhunt", Count: 2}, {Name: "Unending Whisper", Count: 2},
		{Name: "Ureni's Rebuff", Count: 2}, {Name: "Wild Ride", Count: 2}, {Name: "Winternight Stories", Count: 2},
		{Name: "Cancel", Count: 2}, {Name: "Tormenting Voice", Count: 2}, {Name: "Giant Growth", Count: 1}, {Name: "Disfigure", Count: 1},
		{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Akki Rockspeaker", Count: 2}, {Name: "Hill Giant", Count: 1}, {Name: "Fire Elemental", Count: 1},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{ // B: G/U/R
		{Name: "Mammoth Bellow", Count: 2}, {Name: "Roamer's Routine", Count: 2}, {Name: "Synchronized Charge", Count: 2},
		{Name: "Mulch", Count: 2}, {Name: "Satyr Wayfinder", Count: 1}, {Name: "Cathartic Reunion", Count: 1}, {Name: "Tormenting Voice", Count: 1},
		{Name: "Giant Growth", Count: 2}, {Name: "Disfigure", Count: 1},
		{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Lotus Cobra", Count: 3}, {Name: "Mardu Warshrieker", Count: 2},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Fire Elemental", Count: 1}, {Name: "Air Elemental", Count: 1},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 5},
	},
}

type harmonizeStats struct {
	casts, asks, picked, declined int
}

// harmonizeProbe wraps a seeded random policy biased to cast, counting the
// harmonize casts and how each announcement was answered.
type harmonizeProbe struct {
	st   *harmonizeStats
	base Policy
	r    *rand.Rand
}

func (p *harmonizeProbe) pick(c *PickCtx) Canon {
	cp, _ := chooseCode("convoke")
	kind := ActChoose + CanonKind(cp)
	isAsk := false
	var casts []Canon
	for _, a := range c.D.Options {
		if a.Kind() == kind && a.Mode() == 7 {
			isAsk = true
		}
		if a.Kind() == ActCast {
			casts = append(casts, a)
		}
	}
	a := p.base.Pick(c)
	if !isAsk && c.G != nil {
		// gorge's planner prices no harmonize mode, so it offers the cast
		// only when mana floats (Burning-Tree Emissary's or Lotus Cobra's
		// trigger): take it when offered
		var harm []Canon
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			switch {
			case !ok:
			case o.Mode == "harmonize":
				harm = append(harm, x)
			}
		}
		switch {
		case len(harm) > 0 && p.r.IntN(16) != 0:
			a = harm[p.r.IntN(len(harm))]
		case len(casts) > 0 && p.r.IntN(4) != 0:
			a = casts[p.r.IntN(len(casts))]
		}
	}
	if a.Kind() == ActCast && c.G != nil {
		if o, ok := c.G.GorgeOption(a); ok && o.Mode == "harmonize" {
			p.st.casts++
		}
	}
	if isAsk {
		p.st.asks++
		if a.Kind() == ActDone {
			p.st.declined++
		} else if a.Kind() == kind {
			p.st.picked++
		}
	}
	return a
}

// harmonizeSetup builds the matchup's corpus decks and table.
func harmonizeSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("harmonize")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range harmonizeDecks {
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
	tbl, _, _, err := set.Matchup(harmonizeDecks[0], harmonizeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// harmonizeRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func harmonizeRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st harmonizeStats) {
	t.Helper()
	reg, set, tbl, decks := harmonizeSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"harmonize-a", "harmonize-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &harmonizeProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x68726d6e))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
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

func TestHarmonizeLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := harmonizeRun(t, 256, true)
	t.Logf("256 games, harmonize %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.casts, 40, "harmonize casts"},
		{st.picked, 20, "announcements with a creature picked"},
		{st.declined, 10, "declined announcements"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestHarmonizeMutation: a mode-7 pick that subtracts 1 generic instead of
// the creature's power must make the lockstep diverge.
func TestHarmonizeMutation(t *testing.T) {
	dirac.DebugHarmonizeOneGeneric = true
	defer func() { dirac.DebugHarmonizeOneGeneric = false }()
	if diverged, _, _ := harmonizeRun(t, 64, false); !diverged {
		t.Fatal("DebugHarmonizeOneGeneric: no divergence in 64 seeds")
	}
}
