package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestHostedCmdrPodRefusedPairCeiling: of the committed hosted-cmdr slice's
// first 400 four-seat pods (PodAt at seed 1, as kfuzz -kernel hosted-cmdr
// -seats 4 -pods 400 deals them), fewer than 100 are refused for an
// interaction of two decks' cards (cardset.InteractionRefusal, kfuzz's
// "refused-pair"). A refused pod evidences no card. A new pair guard that
// pushes the count back over the ceiling is lifted by modelling it, or by
// treating its slice card as refused (decks.py --also-refuse, the recipe in
// scripts/dirac-evidence.sh).
func TestHostedCmdrPodRefusedPairCeiling(t *testing.T) {
	t.Parallel()
	const pods, ceiling = 400, 100
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
		// a card refused by name would make the pair count a count of nothing
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
