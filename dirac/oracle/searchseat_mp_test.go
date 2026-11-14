//go:build dirac_mp

package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestSearchSeatFourSeatCmd5Pod: a cmd5 Commander pod cut to 50 cards a deck
// (kfuzz -cmd-mini 50) with seat 0 answered by a MaxN dirac/search (16
// simulations) and the other three seats by the heuristic bot ends with no
// divergence or panic, and seat 0 searched. A pod holding Reign of Dragons
// or another two-seat-only card (StallTwoSeat) is skipped.
func TestSearchSeatFourSeatCmd5Pod(t *testing.T) {
	c, names := cmd5MiniCorpus(t, 50)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	pool := make([]PoolDeck, len(names))
	for i, n := range names {
		pool[i] = PoolDeck{Name: n}
	}
	cfg := SearchSeatConfig{Seats: 1, MaxN: true, Sims: 16}
	for i := 0; i < 24; i++ {
		p := PodAt(pool, i, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil || slices.Contains(m.Table, "Reign of Dragons") {
			continue
		}
		rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
		mk, pol := WithSearchSeats(SetSide(set, tbl, rc.Config), HeuristicPolicy(p.Seed), cfg, c.Reg, set)
		k, err := mk(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		res := Lockstep(m, p.Seed, k, pol, rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at %d: %s %s %v", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		by := SearchedBy(pol)
		t.Logf("pod %d: %d decisions, stall %q, searched %v, fallbacks %d", i, res.Decisions, res.Stall, by, PolicyFallbacks(pol))
		if res.Stall == KernelStallPrefix+"twoseat" {
			continue // another two-seat-only card in the pod
		}
		if by[0] == 0 {
			t.Fatalf("pod %d: seat 0 searched no decision", i)
		}
		return
	}
	t.Skip("no cmd5 pod without a two-seat-only card in the first 24")
}
