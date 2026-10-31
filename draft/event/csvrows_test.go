package event_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestDraftDataCSVRowsHasNoHeaderAndMatchesColumns pins the append shape:
// DraftDataCSVRows writes one row per pick with NO header line, and its
// column count is exactly DraftDataCSV's header's (the two are meant for
// the same file, header on the first run, rows on every run after). A
// second header mid-file is not a 17Lands draft_data CSV.
func TestDraftDataCSVRowsHasNoHeaderAndMatchesColumns(t *testing.T) {
	e := newCSVEvent(t)
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("fixture event is in phase %s, want past the draft; the rows below would assert nothing", e.Phase())
	}

	var head bytes.Buffer
	if err := event.DraftDataCSV(e, &head, []string{"Gamma", "Alpha", "Beta"}); err != nil {
		t.Fatalf("DraftDataCSV: %v", err)
	}
	var rows bytes.Buffer
	if err := event.DraftDataCSVRows(e, &rows, []string{"Gamma", "Alpha", "Beta"}); err != nil {
		t.Fatalf("DraftDataCSVRows: %v", err)
	}

	// Precondition: the header export really produced a header and rows.
	headLines := strings.Split(strings.TrimRight(head.String(), "\n"), "\n")
	if len(headLines) < 2 || !strings.HasPrefix(headLines[0], "expansion,event_type") {
		t.Fatalf("DraftDataCSV produced %d lines, want a header line plus rows", len(headLines))
	}
	wantCols := len(strings.Split(headLines[0], ","))

	out := rows.String()
	if strings.Contains(out, "expansion,event_type") {
		t.Fatalf("DraftDataCSVRows wrote a header line:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != len(headLines)-1 {
		t.Fatalf("rows export wrote %d lines, want the %d pick rows the header export carried", len(lines), len(headLines)-1)
	}
	for i, line := range lines {
		if got := len(strings.Split(line, ",")); got != wantCols {
			t.Fatalf("row %d has %d columns, header has %d", i, got, wantCols)
		}
	}
}
