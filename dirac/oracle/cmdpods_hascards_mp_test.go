package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestFourSeatWojekInvestigatesPerOpponent plays pods 1 and 3 of kfuzz
// -kernel cmd5 -seats 4 -cmd-mini 60 -seed 1 -policy random, where Wojek
// Investigator ("investigate once for each opponent who has more cards in
// hand than you", PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GTY)
// resolves in a four-seat pod. The kernel once read the opponents as one
// (vHandSizeOf of RefEachOpponent: a single opponent), so it minted one Clue
// where gorge, counting each opponent (hasPropertyStateBacked), minted two
// (pod 1), and none where gorge minted one (pod 3): projection NInst. Both
// pods must play in lockstep, and some Wojek trigger must mint two or more
// Clues in one resolution, so the per-opponent count is exercised.
func TestFourSeatWojekInvestigatesPerOpponent(t *testing.T) {
	c, names := cmd5MiniCorpus(t, 60)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	pool := make([]PoolDeck, len(names))
	for i, n := range names {
		pool[i] = PoolDeck{Name: n}
	}
	most := 0
	for _, i := range []int{1, 3} {
		p := PodAt(pool, i, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
		rc.Finish = func(g *GorgeSide) {
			run := 0
			for j := range g.E.L.Events {
				e := &g.E.L.Events[j]
				switch e.Kind {
				case events.TokenCreate:
					if e.Text == "c_a_clue_draw" {
						run++
						most = max(most, run)
					}
				case events.Investigate:
				default:
					run = 0
				}
			}
		}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at %d: %s %s %v", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		t.Logf("pod %d (%v): %d decisions, stall %q", i, p.Decks(), res.Decisions, res.Stall)
	}
	if most < 2 {
		t.Fatalf("no resolution minted two Clues (most %d): the per-opponent count went unexercised", most)
	}
}
