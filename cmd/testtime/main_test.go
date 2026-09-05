package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "stream.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res := parseJSON(f)

	host, ok := res["github.com/adams-shaun/gorge/host"]
	if !ok {
		t.Fatal("host package missing from results")
	}
	if host.elapsed != 57.4 {
		t.Errorf("host elapsed = %v, want 57.4", host.elapsed)
	}
	// TestHost/Sub is a subtest and must not count.
	if host.tests != 2 {
		t.Errorf("host tests = %d, want 2 (subtests excluded)", host.tests)
	}

	st, ok := res["github.com/adams-shaun/gorge/state"]
	if !ok {
		t.Fatal("state package missing from results")
	}
	if st.elapsed != 2.1 {
		t.Errorf("state elapsed = %v, want 2.1", st.elapsed)
	}
	if st.tests != 1 {
		t.Errorf("state tests = %d, want 1", st.tests)
	}
}

func TestLoadBudgetExisting(t *testing.T) {
	b := loadBudget(filepath.Join("testdata", "history.md"))
	if b != 60 {
		t.Errorf("loadBudget = %d, want 60", b)
	}
}

func TestLoadBudgetMissing(t *testing.T) {
	if b := loadBudget(filepath.Join("testdata", "does-not-exist.md")); b != -1 {
		t.Errorf("loadBudget(missing) = %d, want -1", b)
	}
}

func TestBudgetForFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	if b := budgetFor(path, 57.4); b != 72 { // ceil(57.4*1.25)=72
		t.Errorf("budgetFor(57.4) = %d, want 72", b)
	}
	if b := budgetFor(path, 1.0); b != 5 { // minimum 5
		t.Errorf("budgetFor(1.0) = %d, want 5", b)
	}
}

func TestWriteHistoryCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")

	writeHistory(path, "github.com/adams-shaun/gorge/host", 60,
		"2026-09-05T20:10Z", "cd7515f", 57.4, 37, "sadams")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		"# Test history — github.com/adams-shaun/gorge/host",
		"budget_s: 60",
		"| date (UTC) | commit | wall_s | tests | runner |",
		"|---|---|---|---|---|",
		"| 2026-09-05T20:10Z | cd7515f | 57.4 | 37 | sadams |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("created file missing %q\n%s", want, s)
		}
	}

	// Append a second row, newest last.
	writeHistory(path, "github.com/adams-shaun/gorge/host", 60,
		"2026-09-06T09:00Z", "abc1234", 58.0, 38, "sadams")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s = string(data)
	if !strings.Contains(s, "| 2026-09-05T20:10Z | cd7515f | 57.4 | 37 | sadams |\n| 2026-09-06T09:00Z | abc1234 | 58.0 | 38 | sadams |") {
		t.Errorf("append did not keep newest last\n%s", s)
	}
	// Budget must not be duplicated on append.
	if strings.Count(s, "budget_s:") != 1 {
		t.Errorf("budget_s duplicated on append\n%s", s)
	}
}
