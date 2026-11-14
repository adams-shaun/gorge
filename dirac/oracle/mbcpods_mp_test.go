//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMBCDraftPodPlays plays four-seat pods of the mbc pool's drafted 50-card
// Commander decks (kfuzz -kernel mbc -seats 4 -allow-unverified), dirac against
// gorge, to their end: the pool's table holds 247 subtypes, past one game
// table's kop.MaxSubtypes, and each pod's game builds its own table (cardset
// matchup; pod 58's needs 68). Four 50-card decks are 200 of the four-seat
// build's MaxObj objects and the rest hold the tokens.
func TestMBCDraftPodPlays(t *testing.T) {
	c, pool, set := mbcSliceCorpus(t)
	if n := len(set.Subtypes()); n <= kop.MaxSubtypes {
		t.Fatalf("precondition: the mbc pool holds %d subtypes, no more than one table's %d", n, kop.MaxSubtypes)
	}
	for _, i := range []int{58, 6} {
		p := PodAt(pool, i, SliceSeed, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d %v: %v", i, p.Decks(), err)
		}
		if !m.Commander || m.NSeat() != 4 {
			t.Fatalf("pod %d: commander %v, %d seats", i, m.Commander, m.NSeat())
		}
		rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		res := Lockstep(m, p.Seed, k, AffordableTargetPolicy(RandomPolicy(p.Seed)), rc)
		CloseSide(k)
		t.Logf("pod %d (%d subtypes, draft pair %v): %d decisions, %d turns, stall %q", i, len(tbl.Subtypes), m.DraftPartners, res.Decisions, res.Turns, res.Stall)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at %d: %s %s %v", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !res.Over || res.Stall != "" {
			t.Errorf("pod %d: over %v, stall %q: want a finished game", i, res.Over, res.Stall)
		}
	}
}
