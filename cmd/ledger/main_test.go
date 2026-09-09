package main

import "testing"

// A lane's meaning depends entirely on counting LEAVES. A parent test that
// fails only because one subtest failed is the same defect counted twice, and
// a lane whose total drifts with the shape of the test tree stops being a
// measurement. Both shapes below appear in the real lane.
func TestLaneEntriesCountsLeavesOnly(t *testing.T) {
	lane := `=== RUN   TestCR601Targets
=== RUN   TestCR601Targets/Bolt
--- FAIL: TestCR601Targets (0.10s)
    --- FAIL: TestCR601Targets/Bolt (0.05s)
    --- PASS: TestCR601Targets/Shock (0.05s)
--- PASS: TestCR602Alone (0.01s)
--- SKIP: TestCR603Skipped (0.00s)
`
	got := laneEntries(lane, map[string]doc{
		"TestCR601Targets": {title: "targets precede payment.", where: "rules/a_test.go:10"},
	})
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3 (two subtest leaves + one standalone); "+
			"the parent TestCR601Targets must not be counted, and a SKIP is not a leaf: %+v", len(got), got)
	}
	byID := map[string]Entry{}
	for _, e := range got {
		byID[e.ID] = e
	}
	if _, ok := byID["TestCR601Targets"]; ok {
		t.Fatal("the parent test was counted as a leaf, double-counting its subtest's defect")
	}
	if e := byID["TestCR601Targets/Bolt"]; e.Status != "open" {
		t.Fatalf("failing leaf status = %q, want open", e.Status)
	}
	// A subtest carries no doc comment of its own; it must inherit the parent
	// func's location so the row still points somewhere.
	if e := byID["TestCR601Targets/Shock"]; e.Status != "closed" || e.Where != "rules/a_test.go:10" {
		t.Fatalf("passing leaf = %+v, want closed at the parent func's location", e)
	}
	if _, ok := byID["TestCR603Skipped"]; ok {
		t.Fatal("a SKIP was recorded; a skipped test measured nothing and is not a ledger row")
	}
}

// The approximations table's description cells carry Forge script fragments,
// and those fragments have their OWN "|" separators. Splitting on "|" and
// taking cells[1] as Where therefore put a script fragment in the location
// column on four real rows. Where and Removed by are the LAST two cells.
func TestApproximationsSplitsOnTheLastTwoCells(t *testing.T) {
	md := "## Known approximations\n\n" +
		"| Stand-in | Where | Removed by |\n|---|---|---|\n" +
		"| plain row | `effects/misc.go:480` | M4 |\n" +
		"| a row whose text has `DB$ Discard | Mode$ TgtChoose` inside it | `effects/cardflow.go` | M5 |\n" +
		"\n## Next section\n" +
		"| not a row | in another table | ignored |\n"
	got := approximations(md)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 — the header, the separator and the next section's table must all be excluded: %+v", len(got), got)
	}
	if got[0].Where != "`effects/misc.go:480`" || got[0].Disposition != "approved — removed by M4" {
		t.Fatalf("plain row = %+v", got[0])
	}
	if got[1].Where != "`effects/cardflow.go`" {
		t.Fatalf("Where = %q, want the LAST-but-one cell; a script fragment in the description "+
			"must not be mistaken for the location", got[1].Where)
	}
	if got[1].Disposition != "approved — removed by M5" {
		t.Fatalf("Disposition = %q", got[1].Disposition)
	}
	if want := "a row whose text has `DB$ Discard | Mode$ TgtChoose` inside it"; got[1].Title != want {
		t.Fatalf("Title = %q, want the description rejoined with its own separators intact", got[1].Title)
	}
}

// A row with fewer than three cells names no location and no milestone, which
// is the one shape the table's header forbids. Dropping it silently is how a
// broken row survives; it is surfaced as open instead.
func TestApproximationsSurfacesAMalformedRow(t *testing.T) {
	md := "## Known approximations\n\n| Stand-in | Where | Removed by |\n|---|---|---|\n" +
		"| a row that lost its columns\n"
	got := approximations(md)
	if len(got) != 1 || got[0].Status != "open" {
		t.Fatalf("got %+v, want one open row flagging the malformed entry", got)
	}
}

func TestFirstSentenceStripsTheGoDocNamePrefix(t *testing.T) {
	if got := firstSentence("TestFoo checks a thing. And more.\n"); got != "checks a thing." {
		t.Fatalf("got %q", got)
	}
	if got := firstSentence(""); got != "" {
		t.Fatalf("empty doc must stay empty, got %q", got)
	}
}
