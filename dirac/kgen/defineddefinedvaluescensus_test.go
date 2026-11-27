package kgen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDefinedValuesCensus lists the Defined$ values the lowerer still
// refuses, with the number of distinct corpus cards blocked on each (the
// catch-all `other:Defined$ "_"` class of the every-blocker census), so the
// next coverage ticket starts from the value it will lower. The rows are
// logged, most-carried first; the asserts pin only what no later value can
// move: the census is non-empty and sorted, and the values the generic
// resolver now lowers are gone from it.
func TestDefinedValuesCensus(t *testing.T) {
	// Sequential, never heavyParallel: the census is read through the
	// package global censusAllBlockers (computeSetlistCensus), and the
	// census tests below assert pinned output under it -- a parallel run
	// could hold the global over their computes.
	std, err := readCardList(filepath.Join("../../", standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	cardsDir := ratchetCards(t)
	censusAllBlockers = true
	defer func() { censusAllBlockers = false }()
	res, err := computeSetlistCensus(testSetlistDir, cardsDir, std)
	if err != nil {
		t.Fatal(err)
	}
	cards := map[string]map[string]bool{}
	for _, r := range res.tsv {
		if r.class != `other:Defined$ "_"` {
			continue
		}
		if cards[r.label] == nil {
			cards[r.label] = map[string]bool{}
		}
		cards[r.label][r.card] = true
	}
	type row struct {
		value string
		n     int
	}
	var rows []row
	for v, cs := range cards {
		rows = append(rows, row{v, len(cs)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].value < rows[j].value
	})
	if len(rows) == 0 {
		t.Fatal("the every-blocker census holds no Defined$ refusal")
	}
	var sb strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&sb, "%4d  %s\n", r.n, r.value)
	}
	t.Logf("remaining Defined$ values (cards, value):\n%s", sb.String())
	for _, gone := range []string{`Defined$ "EnchantedController"`, `Defined$ "TriggeredAttackingPlayer & You"`, `Defined$ "TriggeredSpellAbilityTargets"`} {
		if _, ok := cards[gone]; ok {
			t.Errorf("%s still refused: the generic resolver lowers it", gone)
		}
	}
	for _, kept := range []string{`Defined$ "Convoked"`} {
		if _, ok := cards[kept]; !ok {
			t.Errorf("%s is no longer refused: record it in TestDefinedResolverValuesForms", kept)
		}
	}
}
