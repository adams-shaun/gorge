package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestPodHostedCmdrPodsBuild: the committed hosted-cmdr slice's first eight
// four-seat pods (PodAt at seed 1, as kfuzz -kernel hosted-cmdr -seats 4
// deals them) build with SetMatchupN, so no seat's deck holds a card the
// kernel refuses, and each plays its first 20 decisions in lockstep without
// diverging. A slice built before a refusal fails here at the build, the
// "refused-card" skip kfuzz counts. A pod the kernel refuses for an
// interaction of two decks' cards (cardset.InteractionRefusal, kfuzz's
// "refused-pair") is no stale deck: it is logged and not played, and at
// least one pod must play.
func TestPodHostedCmdrPodsBuild(t *testing.T) {
	base, _ := testCorpus(t)
	set, err := cardset.ByName("hosted-cmdr")
	if err != nil {
		t.Skipf("no hosted-cmdr table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	dir, remove, err := ExtractSlice("hosted-cmdr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	pool, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) < 4 {
		t.Fatalf("slice pool holds %d decks, want at least a four-seat pod's", len(pool))
	}
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	played := 0
	for i := range 8 {
		p := PodAt(pool, i, 1, 4)
		if n := len(p.Decks()); n != 4 {
			t.Fatalf("pod %d: %d decks, want 4", i, n)
		}
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if inter := (*cardset.InteractionRefusal)(nil); errors.As(err, &inter) {
			t.Logf("pod %d (%v): refused-pair: %v", i, p.Decks(), err)
			continue
		}
		if err != nil {
			t.Fatalf("pod %d (%v): %v", i, p.Decks(), err)
		}
		for s, d := range m.Decks {
			if n := len(d); n < 99 {
				t.Fatalf("pod %d seat %d: %d library cards, want a full Commander deck", i, s, n)
			}
		}
		rc := RunConfig{MaxDecisions: 20, MaxTurns: 100}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d (%v): %v", i, p.Decks(), err)
		}
		res := Lockstep(m, p.Seed, k, AffordableTargetPolicy(RandomPolicy(p.Seed)), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at %d: %s %s %v", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Decisions == 0 {
			t.Fatalf("pod %d (%v): no decision played (stall %q)", i, p.Decks(), res.Stall)
		}
		played++
		t.Logf("pod %d (%v): %d decisions, stall %q", i, p.Decks(), res.Decisions, res.Stall)
	}
	if played == 0 {
		t.Fatal("no pod of the first eight played: every one was refused-pair, so the test proves nothing")
	}
}
