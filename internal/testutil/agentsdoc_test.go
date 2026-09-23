package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AGENTS.md's "## Known approximations" table is a CLOSING REGISTER, frozen on
// 2026-09-22. It began as a 13-row audit of where "supported" was not the whole
// truth, and then became the place every landed ticket wrote a paragraph: by
// 2026-09-22 it was ~100 rows and 264 KB, loaded into every agent's context on
// every turn. Every remaining row now has a P1 ticket whose only job is to
// delete it.
//
// This test is the ratchet. It fails on GROWTH and never on shrinkage, so a
// ticket that closes a row only has to delete the row and lower the constant.
const (
	// knownApproximationRows is the number of data rows in the table. Lower it
	// by exactly the number of rows your change deletes. NEVER raise it.
	// The merged table measures 38 rows. Each side deleted disjoint rows and
	// the merge keeps every deletion: this branch's attackprop1 closure
	// (89c77778) and main's cascade1 closure (e46f051d), on top of the
	// token-replacement (bc3f03ab) and replicate-count-bound (0836163f)
	// closures already in the merge base (measured 2026-09-23: merge base
	// 4a7bb2fe = 40 rows, branch 39, main 39, merged 38).
	knownApproximationRows = 38

	// standInCellLimit is the size cap, in bytes, on a row's Stand-in cell: what
	// still deviates today, plus any decision a future implementer must honour.
	// Measurements, corpus counts, "Pinned by Test..." lists, review-round
	// history and "what works" recaps belong in the commit message and the
	// ticket report.
	standInCellLimit = 600

	// knownOversizeRows is how many rows were already over standInCellLimit when
	// the table was frozen. Lower it when you delete one of them; never raise
	// it. A row grown past the cap pushes this over the constant and fails.
	knownOversizeRows = 8
)

func approximationRows(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	var rows []string
	inside := false
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "## Known approximations"):
			inside = true
		case inside && strings.HasPrefix(line, "## "):
			inside = false
		case inside && strings.HasPrefix(line, "| "):
			rows = append(rows, line)
		}
	}
	if len(rows) == 0 {
		t.Fatal("AGENTS.md: no `## Known approximations` table found -- did the heading move?")
	}
	return rows[1:] // drop the header row; the |---|---|---| separator never matches "| "
}

// standInCell is everything but the last two cells (Where, Removed by). Forge
// script fragments in a description carry their own "|", so splitting naively
// would put a script fragment in Where.
func standInCell(row string) string {
	cells := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
	if len(cells) < 3 {
		return strings.TrimSpace(row)
	}
	return strings.TrimSpace(strings.Join(cells[:len(cells)-2], "|"))
}

func TestKnownApproximationsOnlyShrinks(t *testing.T) {
	rows := approximationRows(t)
	if len(rows) > knownApproximationRows {
		t.Errorf("AGENTS.md's Known approximations table has %d rows, above the frozen "+
			"count of %d.\n"+
			"The table is a CLOSING REGISTER: no row may be added. A deviation your "+
			"ticket cannot close goes in the COMMIT MESSAGE and the ticket report, not "+
			"here; if it needs tracking, say so in your report and the operator files a "+
			"ticket.\n"+
			"If you DELETED rows, lower knownApproximationRows in %s to %d.",
			len(rows), knownApproximationRows, "internal/testutil/agentsdoc_test.go", len(rows))
	}
	if len(rows) < knownApproximationRows {
		t.Logf("table is down to %d rows (constant says %d) -- lower knownApproximationRows "+
			"to %d in the same commit that deleted them.",
			len(rows), knownApproximationRows, len(rows))
	}
}

func TestKnownApproximationRowsAreShort(t *testing.T) {
	rows := approximationRows(t)
	var oversize []string
	for _, row := range rows {
		cell := standInCell(row)
		if len(cell) > standInCellLimit {
			head := cell
			if len(head) > 80 {
				head = head[:80]
			}
			oversize = append(oversize, head)
		}
	}
	if len(oversize) > knownOversizeRows {
		t.Errorf("%d Known-approximations rows exceed the %d-byte Stand-in cap, above the "+
			"frozen count of %d. A row may never be grown.\nOversize rows:\n  %s",
			len(oversize), standInCellLimit, knownOversizeRows, strings.Join(oversize, "\n  "))
	}
}
