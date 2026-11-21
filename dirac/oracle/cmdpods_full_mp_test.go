package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestFourSeatCommanderFullPodsLockstep plays four-seat pods of the cmd5
// Commander precons at their full 100 cards (no MiniCommanderDeck cut),
// dirac against gorge, as kfuzz -kernel cmd5 -seats 4 does: no pod
// diverges and none stops on the kernel side (a capacity, two-seat or
// unmodelled stall). A turn or decision cap is the game's length, not a
// stall of the kernel's.
func TestFourSeatCommanderFullPodsLockstep(t *testing.T) {
	base, _ := testCorpus(t)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	dir, remove, err := ExtractSlice("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	pool, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	for _, i := range []int{0, 1, 2, 3} {
		p := PodAt(pool, i, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		for s, d := range m.Decks {
			if n := len(d); n < 99 {
				t.Fatalf("pod %d seat %d: %d library cards, want a full precon", i, s, n)
			}
		}
		rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		res := Lockstep(m, p.Seed, k, AffordableTargetPolicy(RandomPolicy(p.Seed)), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at %d: %s %s %v", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("pod %d (%v): kernel stall %q after %d decisions", i, p.Decks(), res.Stall, res.Decisions)
		}
		t.Logf("pod %d: %d decisions, stall %q", i, res.Decisions, res.Stall)
	}
}
