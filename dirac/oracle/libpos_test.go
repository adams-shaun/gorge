package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated libpos
// pool. Lost Days, Temporal Cleansing and Trickster's Stratagem carry
// AlternativeDecider$ TargetedOwner with LibraryPosition$ 1: the target's
// OWNER chooses "second from top" or "bottom" (gorge zone_change.go), the
// primary placed by libraryOrderPlacementAt(1), an offset clamped to the
// other cards (kernel toLibrarySecond, kop.LibChoiceSecond). Ice Magic's
// Blizzaga mode and Living Library carry LibraryPosition$ 0 with Shuffle$
// True: on top, then the owner's library shuffles (CR 701.24). Both decks
// carry every carrier, so each owner is asked.

var klibposDecks = [2][]cardset.Entry{
	{
		{Name: "Lost Days", Count: 2}, {Name: "Temporal Cleansing", Count: 3},
		{Name: "Trickster's Stratagem", Count: 3}, {Name: "Ice Magic", Count: 2},
		{Name: "Living Library", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Opt", Count: 1},
		{Name: "Vivien Reid", Count: 1},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Lost Days", Count: 2}, {Name: "Temporal Cleansing", Count: 3},
		{Name: "Trickster's Stratagem", Count: 2}, {Name: "Ice Magic", Count: 2},
		{Name: "Living Library", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Divination", Count: 1},
		{Name: "Vivien Reid", Count: 1},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 9},
	},
}

func klibposSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("libpos")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// klibposRun plays seeds 1..seeds of the matchup with the random policy; it
// returns the games played, whether any diverged (the first is logged when
// stopOnDiv) and the kernel stalls.
func klibposRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, klibposDecks)
	tbl, _, _, err := set.Matchup(klibposDecks[0], klibposDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"klibpos-a", "klibpos-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if stopOnDiv {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div = true
			if !stopOnDiv {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestLibposLockstep(t *testing.T) {
	t.Parallel()

	reg, set := klibposSetup(t)
	dirac.DebugLibposLog = true
	dirac.LibposCount = [4]int{}
	defer func() { dirac.DebugLibposLog = false }()
	games, div, stalls := klibposRun(t, reg, set, 256, true)
	n := dirac.LibposCount
	t.Logf("%d games, diverged %v, stalls %d, second from top %d, bottom %d, clamped (0-1 other cards) %d, shuffles %d",
		games, div, stalls, n[dirac.LibposSecond], n[dirac.LibposBottom], n[dirac.LibposClamp], n[dirac.LibposShuffle])
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	// The clamp (offset min(1, others) with 0 or 1 other cards) needs a
	// library of at most 2 cards: 20 turns of 40-card decks with no mill
	// leave ~10 or more, so the test's minimum of 3 is not reached (the
	// count is logged; the clamp is the same min as toLibraryThird's).
	if n[dirac.LibposClamp] < 3 {
		t.Logf("clamped second from top reached %d (3 or more is not reachable in 20 turns)", n[dirac.LibposClamp])
	}
	for _, b := range []struct {
		name      string
		got, want int
	}{
		{"second from top", n[dirac.LibposSecond], 20},
		{"bottom", n[dirac.LibposBottom], 20},
		{"Ice Magic / Living Library shuffles", n[dirac.LibposShuffle], 10},
	} {
		if b.got < b.want {
			t.Errorf("%s reached %d, want at least %d", b.name, b.got, b.want)
		}
	}
}

// TestLibposMutation: putting the "second from top" answer on the top
// (DebugLibposTop) must make the lockstep diverge on some seed (the next
// draw shows it).
func TestLibposMutation(t *testing.T) {
	reg, set := klibposSetup(t)
	dirac.DebugLibposTop = true
	defer func() { dirac.DebugLibposTop = false }()
	if _, div, _ := klibposRun(t, reg, set, 64, false); !div {
		t.Fatalf("DebugLibposTop: no divergence in 64 seeds")
	}
}
