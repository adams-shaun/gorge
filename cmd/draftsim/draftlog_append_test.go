package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDraftsimDraftlogAppendsOneHeader pins the -draftlog append contract:
// two runs into the same file produce ONE header line and one row block per
// event -- a second header mid-file is not a 17Lands draft_data CSV.
func TestDraftsimDraftlogAppendsOneHeader(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	log := filepath.Join(t.TempDir(), "draft.csv")
	o := options{
		set:      "FDN",
		seed:     7,
		events:   1,
		players:  "bot,bot,bot,bot",
		drafter:  "rating-colour",
		builder:  "best23",
		draftlog: log,
		dir:      dir,
	}
	if _, err := run(o, &strings.Builder{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read draftlog: %v", err)
	}
	if _, err := run(o, &strings.Builder{}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read draftlog: %v", err)
	}

	// Precondition: the first run really wrote a header and rows, or the
	// assertion below would pass vacuously on two empty files.
	lines := strings.Split(strings.TrimRight(string(first), "\n"), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "expansion,event_type") {
		t.Fatalf("first run wrote %d lines with first line %q, want a header line plus pick rows", len(lines), lines[0])
	}
	all := strings.Split(strings.TrimRight(string(second), "\n"), "\n")
	headers := 0
	for _, l := range all {
		if strings.HasPrefix(l, "expansion,event_type") {
			headers++
		}
	}
	if headers != 1 {
		t.Fatalf("appended draftlog carries %d header lines, want 1", headers)
	}
	if len(all) != 2*len(lines)-1 {
		t.Fatalf("appended draftlog has %d lines, want the first run's %d plus one header-less run (%d)", len(all), len(lines), 2*len(lines)-1)
	}
}
