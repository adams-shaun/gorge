package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestVoteStoreNumLowers pins the StoreVoteNum$ fixed-list Vote lowering. The
// eight paper carriers lower end to end: each option's sub-program reads its
// own tally as the VoteTally value kind instead of a constant VoteNum SVar,
// and the vote runs every option branch in Choices$ order.
func TestVoteStoreNumLowers(t *testing.T) {
	lowered := []string{
		"Capital Punishment", "Emissary Green", "Fateful Tempest",
		"Lieutenants of the Guard", "Messenger Jays", "Orchard Elemental",
		"Selvala's Stampede", "Travel Through Caradhras",
	}
	lir, ref := lowerCarriers(t, lowered)
	for _, n := range lowered {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
		if sec := section(lir, n); !strings.Contains(sec, "VoteTally") {
			t.Errorf("%s: no VoteTally read lowered:\n%s", n, sec)
		}
	}

	// The eight also build: the new value kind reaches the pool tables.
	kp := kfhBuild(t, lowered, nil)
	for _, n := range lowered {
		if got := refusalText(kp.cards[n]); got != "" {
			t.Errorf("%s: refused: %s", n, got)
		}
	}

	// Capital Punishment has two choices and no tie branch: the Vote op
	// carries the fixed-list mode and the StoreVoteNum bit, and each option
	// branch is flagged so the kernel runs them all in order.
	c := lowerCorpusCard(t, "Capital Punishment")
	vote := -1
	var branches []int
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			switch in.Op {
			case "Vote":
				vote = in.C
			case "Branch":
				branches = append(branches, in.C)
			}
		}
	}
	if vote < 0 {
		t.Fatalf("Capital Punishment: no Vote op\n%s", section(lir, "Capital Punishment"))
	}
	wantVote := 1<<1 | 2<<2 | kop.VoteStore
	if vote != wantVote {
		t.Errorf("Capital Punishment: Vote C=%d, want %d (fixed-list 2 choices, StoreVoteNum)", vote, wantVote)
	}
	if len(branches) != 2 {
		t.Fatalf("Capital Punishment: %d Branch ops, want 2: %v", len(branches), branches)
	}
	for i, c := range branches {
		if want := i | kop.VoteBranchStore; c != want {
			t.Errorf("Capital Punishment: Branch %d C=%d, want store-flagged branch %d", i, c, want)
		}
	}
}
