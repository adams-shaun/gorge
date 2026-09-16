package main

import (
	"reflect"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestCoverageDeckPlanCoversThePoolWithoutRedundantDecks pins coverage mode's
// selection contract against the real corpus and repo lists. The plan must
// cover every registered primitive represented anywhere in the format pool,
// improve on the historical two-deck default, and be byte-stable despite the
// registry's internal maps. A selected deck must add something at its greedy
// turn; otherwise coverage mode would merely repeat a matchup without raising
// the metric it reports.
func TestCoverageDeckPlanCoversThePoolWithoutRedundantDecks(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := testutil.RepoDeckNames()
	first, err := coverageDeckPlan(reg, names)
	if err != nil {
		t.Fatal(err)
	}
	second, err := coverageDeckPlan(reg, names)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("coverage plan changed across identical inputs:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if !sort.StringsAreSorted(first.Names) {
		t.Fatalf("selected names are not sorted: %v", first.Names)
	}
	all, err := deckPrimitiveCoverage(reg, names)
	if err != nil {
		t.Fatal(err)
	}
	if first.Available != all || first.Covered != all {
		t.Errorf("coverage selected %d/%d, pool represents %d; selection must cover every represented registered primitive", first.Covered, first.Available, all)
	}
	baseline, err := deckPrimitiveCoverage(reg, names[:2])
	if err != nil {
		t.Fatal(err)
	}
	if first.Covered <= baseline {
		t.Errorf("coverage selection %d must improve on historical %s,%s baseline %d", first.Covered, names[0], names[1], baseline)
	}
	if len(first.Names) >= len(names) {
		t.Errorf("coverage selection kept every one of %d decks; expected it to drop zero-gain repetitions: %v", len(names), first.Names)
	}
}
