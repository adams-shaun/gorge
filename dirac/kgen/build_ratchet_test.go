package kgen

import (
	"testing"
	"time"
)

// TestCorpusBuildAcceptance is the build-level acceptance ratchet over ALL.txt:
// the cards that both lower and BUILD (counter kinds, cost shapes, filter
// compares are build refusals the LIR pass does not see), as
// `kgen -pool x -card-list ALL.txt` builds them, in process and writing
// nothing. The pin is the ALL row of testdata/build_acceptance.tsv and follows
// the acceptance ratchets' rules (ratchet_test.go): a fall fails, a rise passes
// unless DIRAC_PIN_STRICT=1, and KGEN_REPIN=1 rewrites the row.
func TestCorpusBuildAcceptance(t *testing.T) {
	heavyParallel(t)
	cardsDir := ratchetCards(t)
	pin, ok := readPinTSV(t, buildAcceptanceTSV)["ALL"]
	if !ok {
		t.Fatalf("%s has no ALL row", buildAcceptanceTSV)
	}
	start := time.Now()
	p, err := loadCardList(testSetlistPath(t, "ALL"), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	cv := computeCoverage(p, lw)
	t.Logf("ALL build-level: %d of %d build (%v)", cv.lowered, cv.cards, time.Since(start))
	checkPin(t, buildAcceptanceTSV, "ALL", "ALL build-level", pin, [2]int{cv.lowered, cv.cards})
}
