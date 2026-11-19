package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestTwoSeatCausesArePrep: over every card of the cmd5 and mbc tables,
// each built alone with its tokens, TwoSeatCauses names a cause exactly
// when the kernel's own preparation marks the table two-seat only
// (prepTwoSeat, prepGoad, prepCtrAdded: StallTwoSeat), so the Commander pod
// census (dirac/oracle TestCommanderPodTwoSeatCensus) counts what refuses
// a pod and nothing else.
func TestTwoSeatCausesArePrep(t *testing.T) {
	n, marked := 0, 0
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
		t.Fatal("no card is two-seat only: the comparison is vacuous")
	}
}
