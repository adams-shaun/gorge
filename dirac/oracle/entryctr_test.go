package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: every counter of one
// kind a permanent enters with is ONE put (CR 614.1c, 122.6). Wildgrowth
// Archaic's own converge counters and an earlier Archaic's or Torgal's effect
// counters, a lent body (Thunderous Velocipede) and Endless One's X meet an
// additive modifier (Hardened Scales, Kami of Whispered Hopes, Ozolith) or a
// doubler (Doubling Season).
var entryCtrDecks = [2][]cardset.Entry{
	{ // additive modifiers and effect carriers, dense
		{Name: "Wildgrowth Archaic", Count: 8}, {Name: "Torgal, A Fine Hound", Count: 4},
		{Name: "Hardened Scales", Count: 3}, {Name: "Kami of Whispered Hopes", Count: 3}, {Name: "Ozolith, the Shattered Spire", Count: 2},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Squire", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 6},
	},
	{ // lent bodies, X creatures, and a doubler (never beside an adder: gorge asks the order, the kernel stalls). Mikey & Don stays out: casting a second copy from the library diverges on the static trigger's stack entry with or without this follow (seed 13 of matchup {1,1})
		{Name: "Wildgrowth Archaic", Count: 6}, {Name: "Torgal, A Fine Hound", Count: 3}, {Name: "Thunderous Velocipede", Count: 3},
		{Name: "Doubling Season", Count: 3}, {Name: "Endless One", Count: 3},
		{Name: "Silvercoat Lion", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Squire", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 4},
	},
}

var entryCtrMatchups = [][2]int{{0, 0}, {1, 1}, {0, 1}}

func entryCtrRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("entryctr")
	if err != nil {
		t.Fatal(err)
	}
	for _, mu := range entryCtrMatchups {
		var decks [2][]*cards.Card
		for s, di := range mu {
			for _, e := range entryCtrDecks[di] {
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
		tbl, _, _, err := set.Matchup(entryCtrDecks[mu[0]], entryCtrDecks[mu[1]])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"entryctr-a", "entryctr-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := Policy(RandomPolicy(seed))
			if seed%2 == 1 {
				pol = HeuristicPolicy(seed) // casts what it can: a carrier and a modifier reach the battlefield together
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if report {
					t.Errorf("matchup %v seed %d: kernel stall %s", mu, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				if report {
					lo := max(d.At-12, 0)
					t.Logf("matchup %v seed %d last actions %v", mu, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %v seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				return true, stalls
			}
		}
	}
	return false, stalls
}

func TestEntryctrLockstep(t *testing.T) {
	dirac.EntryMergeStats.Merged.Store(0)
	dirac.EntryMergeStats.Modified.Store(0)
	div, stalls := entryCtrRun(t, 256, true)
	merged, modified := dirac.EntryMergeStats.Merged.Load(), dirac.EntryMergeStats.Modified.Load()
	t.Logf("256 seeds x %d matchups, %d divergences, %d stalls; entries merging 2+ sources %d, of which an additive modifier changed the sum %d",
		len(entryCtrMatchups), b2i(div), stalls, merged, modified)
	if div || stalls > 0 {
		return
	}
	if modified < 20 {
		t.Errorf("entries merged under an additive modifier: %d, want at least 20", modified)
	}
}

// TestEntryctrMutation: separate puts (the pre-5700d1e8e kernel) must make
// the lockstep diverge.
func TestEntryctrMutation(t *testing.T) {
	dirac.DebugEntryCtrSplit = true
	defer func() { dirac.DebugEntryCtrSplit = false }()
	if diverged, _ := entryCtrRun(t, 128, false); !diverged {
		t.Fatal("DebugEntryCtrSplit: no divergence in 128 seeds")
	}
}
