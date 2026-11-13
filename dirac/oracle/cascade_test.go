package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated cascade
// pool, whose only carrier is Bloodbraid Elf (printed K:Cascade). Every
// cascade offer the kernel poses must be gorge's, on the accept and the
// decline arm; the pool's cheap creatures make the found card a free cast and
// its lands/expensive creatures make the run-out (bottom everything) arm.

var cascadeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Bloodbraid Elf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Giant Spider", Count: 2}, {Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
	},
	{
		{Name: "Bloodbraid Elf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Bloodbraid Elf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Gray Ogre", Count: 2}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Bloodbraid Elf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Giant Spider", Count: 2}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 10},
	},
}}

// cascadeProbe counts the cascade election the kernel poses. decline makes
// the policy take the empty answer, exercising the bottom-the-found-card arm.
type cascadeProbe struct {
	n       map[string]int
	base    Policy
	rs      *realSide
	decline bool
}

func (p *cascadeProbe) pick(c *PickCtx) Canon {
	if p.rs.d.Sub == dirac.ChGenCastFree {
		p.n["election offered"]++
	}
	a := p.base.Pick(c)
	if p.decline && p.rs.d.Sub == dirac.ChGenCastFree {
		a = Pack(ActDone, RefNone, RefNone, 0)
	}
	if p.rs.d.Sub == dirac.ChGenCastFree {
		if a.Kind() == ActDone {
			p.n["election declined"]++
		} else {
			p.n["election accepted"]++
		}
	}
	return a
}

func cascadeSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cascade")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// cascadeRun plays seeds 1..seeds of every matchup; it returns the games
// played, whether any diverged (the first is logged) and the stalls.
func cascadeRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, n map[string]int, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	for mi, md := range cascadeDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"cascade-a", "cascade-b"}, decks, 1)
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
			probe := &cascadeProbe{n: n, base: base, rs: k.(*realSide), decline: seed%4 == 0}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div = true
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				if !stopOnDiv {
					return
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	return
}

func TestCascadeLockstep(t *testing.T) {
	t.Parallel()

	reg, set := cascadeSetup(t)
	n := map[string]int{}
	games, div, stalls := cascadeRun(t, reg, set, 512, n, true)
	t.Logf("%d games, diverged %v, stalls %d, counts %v", games, div, stalls, n)
	if div {
		return
	}
	for _, b := range []string{"election offered", "election accepted", "election declined"} {
		if n[b] < 10 {
			t.Errorf("branch %q taken %d times, want at least 10", b, n[b])
		}
	}
}

// TestCascadeMutation: with the native trigger disabled the lockstep must
// diverge (gorge still exiles and offers the free cast).
func TestCascadeMutation(t *testing.T) {
	reg, set := cascadeSetup(t)
	dirac.DebugIgnoreCascade = true
	defer func() { dirac.DebugIgnoreCascade = false }()
	if _, div, _ := cascadeRun(t, reg, set, 64, map[string]int{}, false); !div {
		t.Fatal("native cascade trigger disabled: no divergence in 64 seeds per matchup")
	}
}
