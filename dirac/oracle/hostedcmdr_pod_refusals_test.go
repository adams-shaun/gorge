package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestHostedCmdrPodRefusedPairCeiling: of the committed hosted-cmdr slice's
// first 400 four-seat pods (PodAt at seed 1, as kfuzz -kernel hosted-cmdr
// -seats 4 -pods 400 deals them), fewer than 100 are refused for an
// interaction of two decks' cards (cardset.InteractionRefusal). A refused pod
// evidences no card. A new pair guard that pushes the count back over the
// ceiling is lifted by modelling it, or by treating its slice card as refused
// (decks.py --also-refuse, the recipe in scripts/dirac-evidence.sh).
//
// kfuzz's "refused-pair" is wider than the count here: it also holds the
// layer guard, Room name read and ReduceCost refusals. This test counts none
// of those; any pod refused by one of them, like any other build error, fails
// it outright.
func TestHostedCmdrPodRefusedPairCeiling(t *testing.T) {
	t.Parallel()
	const pods, ceiling = 400, 100
	c, pool, set := hostedCmdrSliceCorpus(t)
	refused, built := 0, 0
	why := map[string]int{}
	for i := range pods {
		p := PodAt(pool, i, 1, 4)
		_, _, err := SetMatchupN(set, c, p.Decks(), 0)
		if inter := (*cardset.InteractionRefusal)(nil); errors.As(err, &inter) {
			refused++
			why[inter.Reason]++
			continue
		}
		// a card refused by name would make the pair count a count of nothing;
		// the untyped older pair guards fail here too (see above)
		if err != nil {
			t.Fatalf("pod %d (%v): %v", i, p.Decks(), err)
		}
		built++
	}
	t.Logf("%d pods: %d built, %d refused-pair %v", pods, built, refused, why)
	if refused >= ceiling {
		t.Errorf("%d of %d pods are refused-pair %v, want under %d", refused, pods, why, ceiling)
	}
}
