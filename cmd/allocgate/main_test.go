package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pprof -top footer that parseTotalMB survives on. Kept as the real tool
// prints it: the parser's contract is to read this exact format, so
// paraphrasing it would test nothing.
const topFooter = "Showing nodes accounting for 4.66GB, 100% of 4.66GB total"

func closeMB(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("%s = %.6f MB, want %.6f MB", what, got, want)
	}
}

// parseTotalMB is the single unit-handling point in the gate. A suffix slip
// here silently moves every budget by a power of 1024, which is why the unit
// cases each assert an exact number and are the mutation-test surface.
func TestParseTotalMBGiga(t *testing.T) {
	got, err := parseTotalMB(topFooter)
	if err != nil {
		t.Fatal(err)
	}
	// 4.66 * 2^30 bytes = 4.66 * 2^10 MB. If the parser read GB as MB the
	// result would be 4.66, over a thousand times too small.
	closeMB(t, "GB total", got, 4.66*1024)
}

func TestParseTotalMBMega(t *testing.T) {
	got, err := parseTotalMB("Showing nodes accounting for 60.17MB, 100% of 60.17MB total")
	if err != nil {
		t.Fatal(err)
	}
	closeMB(t, "MB total", got, 60.17)
}

func TestParseTotalMBKilo(t *testing.T) {
	got, err := parseTotalMB("Showing nodes accounting for 512.00kB, 100% of 512.00kB total")
	if err != nil {
		t.Fatal(err)
	}
	closeMB(t, "kB total", got, 0.5)
}

func TestParseTotalMBBytes(t *testing.T) {
	got, err := parseTotalMB("Showing nodes accounting for 1024B, 100% of 1024B total")
	if err != nil {
		t.Fatal(err)
	}
	closeMB(t, "B total", got, 1.0/1024)
}

func TestParseTotalMBIntegerValue(t *testing.T) {
	got, err := parseTotalMB("Showing nodes accounting for 170260MB, 100% of 170260MB total")
	if err != nil {
		t.Fatal(err)
	}
	closeMB(t, "integer MB total", got, 170260)
}

func TestParseTotalMBRejectsMissingTotal(t *testing.T) {
	if _, err := parseTotalMB("Showing nodes accounting for 4.66GB of nothing"); err == nil {
		t.Fatal("expected an error when the 'total' footer is absent")
	}
}

func TestParseTotalMBRejectsBadSuffix(t *testing.T) {
	// A value with no unit suffix must not be read as bytes: an unlabeled
	// "4.66 total" is pprof's count-axis, not an allocation, and trusting it
	// would make the gate meaningless.
	if _, err := parseTotalMB("Showing nodes accounting for 4.66, 100% of 4.66 total"); err == nil {
		t.Fatal("expected an error when the total has no unit suffix")
	}
}

func TestBudget1_25(t *testing.T) {
	if got := budget1_25(5757); got != 7197 { // ceil(5757 * 1.25) = 7196.25 → 7197
		t.Errorf("budget1_25(5757) = %d, want 7197", got)
	}
	if got := budget1_25(40); got != 50 {
		t.Errorf("budget1_25(40) = %d, want 50", got)
	}
	if got := budget1_25(2); got != 16 { // floor for sampling-noisy tiny packages
		t.Errorf("budget1_25(2) = %d, want 16", got)
	}
	if got := budget1_25(0); got != 16 {
		t.Errorf("budget1_25(0) = %d, want 16", got)
	}
}

func TestBudgetsForFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ALLOC_HISTORY.md")
	b := budgetsFor(path, measure{rssMB: 5757, allocMB: 170260})
	if b.rssMB != 7197 {
		t.Errorf("fresh rss budget = %d, want 7197", b.rssMB)
	}
	if b.allocMB != int(math.Ceil(170260*1.25)) {
		t.Errorf("fresh alloc budget = %d, want ceil(170260*1.25)=%d", b.allocMB, int(math.Ceil(170260*1.25)))
	}
}

func TestBudgetsForReusesRecordedRatchet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ALLOC_HISTORY.md")
	content := "# Allocation history — x\n\nalloc_budget_mb: 30000\nrss_budget_mb: 9000\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// A huge measurement must not loosen the recorded ratchet.
	b := budgetsFor(path, measure{rssMB: 100000, allocMB: 1000000})
	if b.rssMB != 9000 {
		t.Errorf("reused rss budget = %d, want recorded 9000", b.rssMB)
	}
	if b.allocMB != 30000 {
		t.Errorf("reused alloc budget = %d, want recorded 30000", b.allocMB)
	}
}

func TestLoadBudgets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ALLOC_HISTORY.md")
	content := "# Allocation history — y\n\nalloc_budget_mb: 30000\nrss_budget_mb: 9000\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	b := loadBudgets(path)
	if b.allocMB != 30000 || b.rssMB != 9000 {
		t.Errorf("loadBudgets = %+v, want {30000 9000}", b)
	}

	missing := loadBudgets(filepath.Join(dir, "does-not-exist.md"))
	if missing.allocMB != -1 || missing.rssMB != -1 {
		t.Errorf("loadBudgets(missing) = %+v, want {-1 -1}", missing)
	}
}

func TestWriteHistoryCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ALLOC_HISTORY.md")

	writeHistory(path, "github.com/adams-shaun/gorge/host",
		budgets{allocMB: 212800, rssMB: 7197},
		"2026-09-06T20:10Z", "cd7515f",
		measure{rssMB: 5757, allocMB: 170260}, "sadams")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		"# Allocation history — github.com/adams-shaun/gorge/host",
		"alloc_budget_mb: 212800",
		"rss_budget_mb: 7197",
		"| date (UTC) | commit | rss_mb | alloc_mb | runner |",
		"|---|---|---|---|---|",
		"| 2026-09-06T20:10Z | cd7515f | 5757 | 170260 | sadams |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("created file missing %q\n%s", want, s)
		}
	}

	// Append a second row, newest last.
	writeHistory(path, "github.com/adams-shaun/gorge/host",
		budgets{allocMB: 212800, rssMB: 7197},
		"2026-09-07T09:00Z", "abc1234",
		measure{rssMB: 5800, allocMB: 171000}, "sadams")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s = string(data)
	if !strings.Contains(s, "| 2026-09-06T20:10Z | cd7515f | 5757 | 170260 | sadams |\n| 2026-09-07T09:00Z | abc1234 | 5800 | 171000 | sadams |") {
		t.Errorf("append did not keep newest last\n%s", s)
	}
	// Budgets must not be duplicated on append.
	if strings.Count(s, "alloc_budget_mb:") != 1 || strings.Count(s, "rss_budget_mb:") != 1 {
		t.Errorf("budget keys duplicated on append\n%s", s)
	}
}
