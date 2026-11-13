package kgen

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestTwoSeatReasons ranks the reasons that hold supported cards to two
// seats (lCard.TwoSeatWhy): per reason, the supported cards of the whole
// corpus it marks and the Standard cards among them. It logs the ranking
// and checks only that every two-seat card names a reason.
func TestTwoSeatReasons(t *testing.T) {
	heavyParallel(t)
	cardsDir := ratchetCards(t)
	s, err := embeddedSupport()
	if err != nil {
		t.Fatal(err)
	}
	codes, err := readCardList(filepath.Join(testRoot, standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	std := map[string][]string{} // card -> Standard sets
	byReason := map[string]map[string]bool{}
	for _, list := range append(codes, "ALL") {
		p, err := loadCardList(testSetlistPath(t, list), cardsDir)
		if err != nil {
			t.Fatal(err)
		}
		lw := lowerPool(p)
		if _, _, _, err := buildPool(lw); err != nil {
			t.Fatalf("%s: %v", list, err)
		}
		for _, c := range lw.cards {
			if st, _ := statusOf(c, s); st != cardset.StatusSupported || !c.TwoSeat {
				continue
			}
			if len(c.TwoSeatWhy) == 0 {
				t.Errorf("%s: two-seat only with no reason", c.Name)
			}
			if list != "ALL" {
				if !slices.Contains(std[c.Name], list) {
					std[c.Name] = append(std[c.Name], list)
				}
				continue
			}
			for _, r := range c.TwoSeatWhy {
				if byReason[r] == nil {
					byReason[r] = map[string]bool{}
				}
				byReason[r][c.Name] = true
			}
		}
	}
	reasons := make([]string, 0, len(byReason))
	for r := range byReason {
		reasons = append(reasons, r)
	}
	slices.SortFunc(reasons, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(byReason[b]), len(byReason[a])), strings.Compare(a, b))
	})
	var b strings.Builder
	b.WriteString("| reason | cards | Standard cards |\n|---|---:|---|\n")
	for _, r := range reasons {
		var names []string
		for n := range byReason[r] {
			if sets := std[n]; len(sets) > 0 {
				names = append(names, fmt.Sprintf("%s (%s)", n, strings.Join(sets, ",")))
			}
		}
		slices.Sort(names)
		fmt.Fprintf(&b, "| %s | %d | %s |\n", r, len(byReason[r]), strings.Join(names, "; "))
	}
	t.Logf("two-seat reasons over the supported cards:\n%s", b.String())
}
