package oracle

import "testing"

// TestPrintedDelveTableIsShared: printedDelveSet builds a kernel table with kgen,
// the most expensive step of replaying a custom repro, and its output depends only
// on the corpus and the sorted cards its decks hold (11-27, the oracle lane budget:
// TestRefusedEntriesStillDiverge rebuilt an entry's table with every run). Two
// calls with the same decks must return the identical Set: a fresh build per call
// (the pre-fix behaviour) returns two distinct tables and this fails.
func TestPrintedDelveTableIsShared(t *testing.T) {
	decks := [2][]string{
		{"Gurmag Angler", "Murderous Cut", "Swamp"},
		{"Murderous Cut", "Island"},
	}
	first := printedDelveSet(t, decks)
	if first == nil {
		t.Fatal("no table built")
	}
	// the precondition the identity assertion depends on: the same decks, asked twice
	again := printedDelveSet(t, decks)
	if again != first {
		t.Errorf("the same decks built two tables: kgen ran twice, the lane pays for both")
	}
	// and a different deck list must not hit the same entry
	other := printedDelveSet(t, [2][]string{{"Gurmag Angler", "Swamp"}, {"Murderous Cut", "Island"}})
	if other == first {
		t.Errorf("different card sets share one table: the cache key ignores the decks")
	}
}
