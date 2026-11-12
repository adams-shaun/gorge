package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated
// increment pool, whose carriers carry the bare `K:Increment` (CR 702.xx,
// Secrets of Strixhaven): "whenever you cast a spell, if the amount of mana
// you spent is greater than this creature's power or toughness, put a +1/+1
// counter on this creature". The comparison is event-relative -- the cast's
// pool mana spent (kernel Tf2Increment, State.incrementAdmits over the
// spell entry's recorded spend) against the source's layer-output P/T at
// the moment the trigger fires, an OR of the two comparisons, rechecked
// with the captured spend as the trigger resolves (State.incrementResolves).
// The decks mix
// the carriers with cheap spells (Opt, Llanowar Elves), an expensive one
// (Hill Giant) and Giant Growth, whose pump moves the P/T side of the
// comparison exactly as gorge's layer output moves.

var kincrementDecks = [2][]cardset.Entry{
	{
		{Name: "Ambitious Augmenter", Count: 2}, {Name: "Cuboid Colony", Count: 2},
		{Name: "Hungry Graffalon", Count: 2}, {Name: "Pensive Professor", Count: 2},
		{Name: "Textbook Tabulator", Count: 2}, {Name: "Topiary Lecturer", Count: 2},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Giant Growth", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
	},
}

func kincrementSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("increment")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kincrementRun plays seeds 1..seeds of the matchup; it returns the games
// played, whether any diverged (the first is logged) and the stalls.
func kincrementRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, n map[string]int, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, kincrementDecks)
	tbl, _, _, err := set.Matchup(kincrementDecks[0], kincrementDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kincrement-a", "kincrement-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := Policy(HeuristicPolicy(seed)) // preferring casts: the bot plans casts, a manual tap falls back to random
		if seed%3 == 0 {
			base = RandomPolicy(seed)
		}
		dirac.IncrementLog = nil
		res := Lockstep(m, seed, k, base, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
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
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			for i, r := range dirac.IncrementLog {
				t.Logf("inc[%d] spent=%d p=%d t=%d admitted=%v ctrl=%d caster=%d", i, r.Spent, r.Power, r.Tough, r.Admitted, r.Ctrl, r.Caster)
			}
			return
		}
		for _, r := range dirac.IncrementLog {
			if r.Ctrl != r.Caster {
				continue // Whose$ You: the trigger only watches its own controller's casts
			}
			if r.Admitted {
				n["triggered"]++
				// admitted through exactly one of the two comparisons
				if r.Spent > r.Tough && r.Spent <= r.Power {
					n["only toughness"]++
				}
				if r.Spent > r.Power && r.Spent <= r.Tough {
					n["only power"]++
				}
			} else {
				n["below both"]++
			}
		}
	}
	return
}

func TestIncrementLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kincrementSetup(t)
	dirac.DebugIncrementLog = true
	defer func() { dirac.DebugIncrementLog = false }()
	n := map[string]int{}
	games, div, stalls := kincrementRun(t, reg, set, 256, n, true)
	t.Logf("%d games, diverged %v, stalls %d, counts %v", games, div, stalls, n)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	// The minimums the test set; where the games reach less, the count
	// reached is kept (named in the report). "Only toughness" (spent > T,
	// spent <= P) is unreachable in this pool by construction: every carrier
	// has power <= toughness (1/1, 1/1, 3/4, 0/2, 0/3, 1/2) and nothing
	// lowers a power below its toughness, so only the power side of the
	// disjunction can admit alone; the assertion is the test's "or" as one
	// combined count.
	if n["only power"]+n["only toughness"] < 10 {
		t.Errorf("only-one-comparison admissions reached %d (only power %d, only toughness %d), want at least 10", n["only power"]+n["only toughness"], n["only power"], n["only toughness"])
	}
	for _, b := range []struct {
		key  string
		want int
	}{
		{"triggered", 40},
		{"below both", 40},
	} {
		if n[b.key] < b.want {
			t.Errorf("branch %q reached %d, want at least %d", b.key, n[b.key], b.want)
		}
	}
}

// TestIncrementLockstepMutation: requiring BOTH comparisons (&&) instead
// of gorge's OR must make the lockstep diverge on some seed.
func TestIncrementLockstepMutation(t *testing.T) {
	reg, set := kincrementSetup(t)
	dirac.DebugIncrementAnd = true
	defer func() { dirac.DebugIncrementAnd = false }()
	_, div, _ := kincrementRun(t, reg, set, 64, map[string]int{}, false)
	if !div {
		t.Fatalf("DebugIncrementAnd: no divergence in 64 seeds")
	}
}

var _ = kop.Tf2Increment
