package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: Torgal, A Fine Hound and Wildgrowth Archaic,
// whose SpellCast triggers make the cast creature enter with extra +1/+1
// counters (DB$ Effect | ReplacementEffects$ <Moved -> PutCounter ETB> |
// ExileOnMoved$ Stack), against counterspells (a spell countered with the
// trigger on the stack gets nothing) and a counter doubler (Hardened Scales).
var etbCtrDecks = [3][]cardset.Entry{
	{ // green-white: both carriers, Humans, Dogs and Wolves
		{Name: "Torgal, A Fine Hound", Count: 4}, {Name: "Wildgrowth Archaic", Count: 5}, {Name: "Hardened Scales", Count: 1},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Squire", Count: 3}, {Name: "Border Guard", Count: 3},
		{Name: "Knight of New Benalia", Count: 2}, {Name: "Thornhide Wolves", Count: 1}, {Name: "Thraben Purebloods", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Plains", Count: 7}, {Name: "Blossoming Sands", Count: 2},
	},
	{ // two carriers in play at once: every creature cast raises two triggers
		{Name: "Wildgrowth Archaic", Count: 12}, {Name: "Torgal, A Fine Hound", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Squire", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Blossoming Sands", Count: 2},
	},
	{ // blue-green-red: Wildgrowth, Torgal, counterspells
		{Name: "Wildgrowth Archaic", Count: 4}, {Name: "Torgal, A Fine Hound", Count: 2}, {Name: "Cancel", Count: 3}, {Name: "Essence Scatter", Count: 3},
		{Name: "Talas Merchant", Count: 2}, {Name: "Jhessian Lookout", Count: 2}, {Name: "Tolarian Scholar", Count: 2},
		{Name: "Fugitive Wizard", Count: 1}, {Name: "Russet Wolves", Count: 1}, {Name: "Thornhide Wolves", Count: 1},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Order of the Sacred Bell", Count: 1},
		{Name: "Forest", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 3},
		{Name: "Thornwood Falls", Count: 1}, {Name: "Rugged Highlands", Count: 1}, {Name: "Swiftwater Cliffs", Count: 1},
	},
}

func etbCtrRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effetbctr")
	if err != nil {
		t.Fatal(err)
	}
	for _, mu := range etbCtrMatchups {
		var decks [2][]*cards.Card
		for s, di := range mu {
			for _, e := range etbCtrDecks[di] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("deck %d: %d cards", di, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(etbCtrDecks[mu[0]], etbCtrDecks[mu[1]])
		if err != nil {
			t.Fatal(err)
		}
		if d, st := etbCtrSeeds(t, reg, set, tbl, decks, seeds, report); d {
			return true, stalls + st
		} else {
			stalls += st
		}
	}
	return false, stalls
}

// etbCtrMatchups: the green-white deck against the counterspell deck, and the green-white
// and carrier-dense decks against themselves (both players' carriers cast creatures).
var etbCtrMatchups = [][2]int{{0, 2}, {0, 0}, {1, 1}}

func etbCtrSeeds(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *dirac.Table, decks [2][]*cards.Card, seeds uint64, report bool) (diverged bool, stalls int) {
	t.Helper()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"etbctr-a", "etbctr-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := Policy(RandomPolicy(seed))
		if seed%4 == 3 {
			pol = HeuristicPolicy(seed) // casts what it can: two carriers reach the battlefield together
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return true, stalls
		}
	}
	return false, stalls
}

func TestEffetbctrLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.EffEtbStats.Applied.Store(0)
	dirac.EffEtbStats.Big.Store(0)
	// Two effects upgrading one entry stall the kernel loudly (not modelled).
	div, stalls := etbCtrRun(t, 256, true)
	applied, big := dirac.EffEtbStats.Applied.Load(), dirac.EffEtbStats.Big.Load()
	t.Logf("256 seeds, %d divergences, %d stalls; effect applications %d (of 2+ counters %d)", b2i(div), stalls, applied, big)
	if div {
		return
	}
	for _, c := range []struct {
		s        string
		got, min int64
	}{
		{"applications", applied, 200},
		{"applications of 2+ counters", big, 60},
	} {
		if c.got < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.got, c.min)
		}
	}
}

// TestEffetbctrMutation: a permanent entering one counter short of the
// effect's count must make the lockstep diverge.
func TestEffetbctrMutation(t *testing.T) {
	dirac.DebugEffEtbCtr = true
	defer func() { dirac.DebugEffEtbCtr = false }()
	if diverged, _ := etbCtrRun(t, 128, false); !diverged {
		t.Fatal("DebugEffEtbCtr: no divergence in 128 seeds")
	}
}
