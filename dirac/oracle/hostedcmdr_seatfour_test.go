package oracle

import (
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHostedCmdrPodsSeatFour: none of the committed hosted-cmdr slice's first
// 400 four-seat pods (PodAt at seed 1, as kfuzz -kernel hosted-cmdr -seats 4
// -pods 400 deals them) stalls at its start in a four-seat game:
// kop.PairGuardN(table, 4) is nil for every pod that built. A pod the kernel
// refused for an interaction of two decks' cards (cardset.InteractionRefusal)
// has no table to stall: it is skipped, and TestHostedCmdrPodRefusedPairCeiling
// counts those. A capacity stall at a pod's start evidences no card, and every
// seat of such a pod plays nothing, so the whole pod is lost to the census;
// each reason that reaches this list is a two-seat-only read the four-seat
// build keeps (dirac.TwoSeatCauses), not a declared limit.
func TestHostedCmdrPodsSeatFour(t *testing.T) {
	t.Parallel()
	const pods = 400
	c, pool, set := hostedCmdrSliceCorpus(t)
	stalled := 0
	shown := map[string]bool{}
	for i := range pods {
		p := PodAt(pool, i, 1, 4)
		if n := len(p.Decks()); n != 4 {
			t.Fatalf("pod %d: %d decks, want 4", i, n)
		}
		_, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if inter := (*cardset.InteractionRefusal)(nil); errors.As(err, &inter) {
			continue // refused-pair: no table, no start stall (the ceiling test counts it)
		}
		if err != nil {
			t.Fatalf("pod %d (%v): %v", i, p.Decks(), err)
		}
		if r := kop.PairGuardN(tbl, 4); r != nil {
			stalled++
			names := make([]string, len(r.Cards))
			for j, id := range r.Cards {
				if int(id) < len(tbl.Cards) {
					names[j] = tbl.Cards[id].Name
				}
			}
			key := r.Reason + "\x00" + r.Why + "\x00" + strings.Join(names, ",")
			if !shown[key] {
				shown[key] = true
				t.Errorf("pod %d (%v): four-seat start stall %q (%s) naming %v", i, p.Decks(), r.Reason, r.Why, names)
			}
		}
	}
	t.Logf("%d pods: %d four-seat start stalls", pods, stalled)
	if stalled != 0 {
		t.Errorf("%d of %d pods hold a four-seat start stall, want 0", stalled, pods)
	}
}
