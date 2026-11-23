package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoSeatCausesArePrep: over every card of the cmd5 and mbc tables,
// each built alone with its tokens, TwoSeatCauses names a cause exactly
// when the kernel's own preparation marks the table two-seat only
// (prepTwoSeat, prepGoad, prepCtrAdded: StallTwoSeat), so the Commander pod
// census (dirac/oracle TestCommanderPodTwoSeatCensus) counts what refuses
// a pod and nothing else.
func TestTwoSeatCausesArePrep(t *testing.T) {
	n, marked := 0, 0
	var last *kop.Table // the last pool card's table, for the witness below
	for _, name := range []string{"cmd5", "mbc"} {
		set, err := cardset.ByName(name)
		if err != nil {
			t.Skipf("no %s table (make dirac-tables): %v", name, err)
		}
		set = cardset.AllowUnverified(set)
		for i, card := range set.Names()[:set.NumCards()] {
			if !set.Supported()[i] {
				continue
			}
			tb, _, err := set.MatchupNInert([][]cardset.Entry{{{Name: card, Count: 1}}})
			if err != nil {
				continue // a card the set does not play
			}
			last = tb
			causes := TwoSeatCauses(tb)
			any := false
			for _, c := range causes {
				any = any || len(c) > 0
			}
			if only := TwoSeatOnly(tb); only != any {
				t.Errorf("%s %s: preparation two-seat only %v, TwoSeatCauses %v", name, card, only, causes)
			}
			n++
			if any {
				marked++
			}
		}
	}
	t.Logf("%d cards, %d two-seat only", n, marked)
	if marked == 0 {
		// The pod-limited retirement (TestPodLimitedRemainderRefusedByName)
		// emptied the corpus's two-seat-only set, so the comparison above can
		// no longer catch a preparation that never marks: no card has a cause
		// to name. A synthetic marked card beside the last pool's cards keeps
		// the marking real: the preparation marks the table and TwoSeatCauses
		// names its cause, exactly as it would for a corpus card
		// (TestCardTwoSeat is the card-level witness).
		w := *last
		w.Cards = append(slices.Clone(last.Cards), kop.CardDef{TwoSeat: true})
		if !TwoSeatOnly(&w) {
			t.Error("a table holding a two-seat-marked card does not stall past two seats")
		}
		if got := TwoSeatCauses(&w); !slices.Equal(got[len(got)-1], []string{TwoSeatKgen}) {
			t.Errorf("the marked card's causes %v, want [%s]", got[len(got)-1], TwoSeatKgen)
		}
	}
}
