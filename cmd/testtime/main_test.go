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

func TestPackagesForFiles(t *testing.T) {
	pkgs := []pkgInfo{
		{importPath: "example.com/gorge/host", dir: "host", hasTests: true},
		{importPath: "example.com/gorge/state", dir: "state", hasTests: true},
		{importPath: "example.com/gorge/cmd/testtime", dir: "cmd/testtime", hasTests: true},
		{importPath: "example.com/gorge/cmd/forgec", dir: "cmd/forgec", hasTests: false},
	}

	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "empty input",
			files: nil,
			want:  nil,
		},
		{
			name:  "non-go files only",
			files: []string{"host/README.md", "state/doc.txt"},
			want:  []string{"example.com/gorge/host", "example.com/gorge/state"},
		},
		{
			name:  "several files in one package",
			files: []string{"host/a.go", "host/b.go", "host/a_test.go"},
			want:  []string{"example.com/gorge/host"},
		},
		{
			name:  "files in several packages",
			files: []string{"host/a.go", "state/b.go"},
			want:  []string{"example.com/gorge/host", "example.com/gorge/state"},
		},
		{
			name:  "file in a package with no _test.go",
			files: []string{"cmd/forgec/main.go"},
			want:  []string{"example.com/gorge/cmd/forgec"},
		},
		{
			name:  "file not in any known package",
			files: []string{"vendor/unknown/x.go"},
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packagesForFiles(tt.files, pkgs)
			if len(got) != len(tt.want) {
				t.Fatalf("packagesForFiles(%v) = %v, want %v", tt.files, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("packagesForFiles(%v)[%d] = %q, want %q", tt.files, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestPackagesForFilesSkipsToolArtifact is the mutation surface for the
// feedback-loop fix. The gate tools append a TEST_HISTORY.md / ALLOC_HISTORY.md
// row to every package they measure; if a commit fails after they measure, that
// bookkeeping stays staged and dirty, and the next -changed run must NOT treat
// the package as changed because of it (doing so would re-measure everything
// under self-inflicted load and report wall times it did not earn). A package
// whose only staged change is one of these history files is not a changed
// package.
func TestPackagesForFilesSkipsToolArtifact(t *testing.T) {
	pkgs := []pkgInfo{
		{importPath: "example.com/gorge/host", dir: "host", hasTests: true},
		{importPath: "example.com/gorge/state", dir: "state", hasTests: true},
	}

	// Bookkeeping alone must never select a package.
	if got := packagesForFiles([]string{"host/TEST_HISTORY.md"}, pkgs); len(got) != 0 {
		t.Errorf("one TEST_HISTORY.md on its own selected %v, want none (the tool's own bookkeeping is not a change)", got)
	}
	if got := packagesForFiles([]string{"state/ALLOC_HISTORY.md"}, pkgs); len(got) != 0 {
		t.Errorf("one ALLOC_HISTORY.md on its own selected %v, want none", got)
	}

	// Neither tool's bookkeeping selects, even where a real file does.
	if got := packagesForFiles([]string{"host/testtime.go", "host/TEST_HISTORY.md", "host/ALLOC_HISTORY.md"}, pkgs); len(got) != 1 || got[0] != "example.com/gorge/host" {
		t.Errorf("bookkeeping beside host/testtime.go changed selection to %v, want just host", got)
	}
}

// TestNonArtifactsWedged is the mutation surface for the wedged-state fix: a
// staging that carries ONLY the gates' own bookkeeping is a wedge, and the
// changed-file list it yields must be empty so -changed measures nothing and
// reports the wedge.
func TestNonArtifactsWedged(t *testing.T) {
	staged := []string{"host/TEST_HISTORY.md", "state/ALLOC_HISTORY.md"}
	if got := nonArtifacts(staged); len(got) != 0 {
		t.Errorf("nonArtifacts(bookkeeping-only) = %v, want empty (a wedge must measure nothing)", got)
	}

	touched := nonArtifacts([]string{"host/a.go", "host/TEST_HISTORY.md"})
	if len(touched) != 1 || touched[0] != "host/a.go" {
		t.Errorf("nonArtifacts dropped a real file: %v", touched)
	}
}

func TestWriteHistoryCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")

	writeHistory(path, "github.com/adams-shaun/gorge/host", 60,
		"2026-09-05T20:10Z", "cd7515f", 57.4, 37, 2, "sadams")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		"# Test history — github.com/adams-shaun/gorge/host",
		"budget_s: 60",
		"| date (UTC) | commit | wall_s | tests | skipped | runner |",
		"|---|---|---|---|---|---|",
		"| 2026-09-05T20:10Z | cd7515f | 57.4 | 37 | 2 | sadams |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("created file missing %q\n%s", want, s)
		}
	}

	// Append a second row, newest last.
	writeHistory(path, "github.com/adams-shaun/gorge/host", 60,
		"2026-09-06T09:00Z", "abc1234", 58.0, 38, 3, "sadams")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s = string(data)
	if !strings.Contains(s, "| 2026-09-05T20:10Z | cd7515f | 57.4 | 37 | 2 | sadams |\n| 2026-09-06T09:00Z | abc1234 | 58.0 | 38 | 3 | sadams |") {
		t.Errorf("append did not keep newest last\n%s", s)
	}
	// Budget must not be duplicated on append.
	if strings.Count(s, "budget_s:") != 1 {
		t.Errorf("budget_s duplicated on append\n%s", s)
	}
}

func TestWriteHistoryMigratesFiveColumnHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	oldRow := "| 2026-09-05T20:10Z | cd7515f | 57.4 | 37 | sadams |"
	old := "# Test history — x\n\nbudget_s: 60\n\n| date (UTC) | commit | wall_s | tests | runner |\n|---|---|---|---|---|\n" + oldRow + "\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	writeHistory(path, "x", 60, "2026-09-06T09:00Z", "abc1234", 58.0, 38, 3, "sadams")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|") {
		t.Errorf("five-column header was not migrated\n%s", s)
	}
	if !strings.Contains(s, oldRow) {
		t.Errorf("historical data row was altered\n%s", s)
	}
	if !strings.HasSuffix(s, "| 2026-09-06T09:00Z | abc1234 | 58.0 | 38 | 3 | sadams |\n") {
		t.Errorf("six-column row not appended under migrated header\n%s", s)
	}
}

// TestParseJSONCountsSkipped is the mutation surface for the vacuous-measurement
// fix: a test that SKIPPED its body must be counted as skipped, alongside (not
// in place of) the run count, so a run whose corpus-backed tests skipped can be
// told apart from one that really ran.
func TestParseJSONCountsSkipped(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "stream_skip.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res := parseJSON(f)

	pkg, ok := res["github.com/adams-shaun/gorge/rules"]
	if !ok {
		t.Fatal("rules package missing from results")
	}
	// Top-level run events (TestRepoDecks/2 is a subtest and must not count):
	// TestEveryRepoDeck, TestRepoDecks, TestHeads, TestAcceptance = 4.
	if pkg.tests != 4 {
		t.Errorf("rules tests = %d, want 4 (subtests excluded)", pkg.tests)
	}
	// Top-level skip events (TestRepoDecks/2 is a subtest and must not count):
	// TestEveryRepoDeck, TestRepoDecks = 2.
	if pkg.skipped != 2 {
		t.Errorf("rules skipped = %d, want 2 (subtests excluded)", pkg.skipped)
	}
	if pkg.elapsed != 3.6 {
		t.Errorf("rules elapsed = %v, want 3.6", pkg.elapsed)
	}
}

// TestHistorySkipBaselineNoHistory pins that a package with no prior history
// (no file, or a file with no rows) has no baseline and is therefore never
// anomalous: a first measurement must be recorded, not refused.
func TestHistorySkipBaselineNoHistory(t *testing.T) {
	have, base := historySkipBaseline(filepath.Join("testdata", "does-not-exist.md"))
	if have {
		t.Error("missing history file reported haveHistory=true, want false")
	}
	if base != 0 {
		t.Errorf("baseline for missing file = %v, want 0", base)
	}

	// A file that exists but holds no data rows is the same "no prior
	// measurement" state, and must also never be anomalous.
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	onlyHeader := "# Test history — x\n\nbudget_s: 5\n\n| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|\n"
	if err := os.WriteFile(path, []byte(onlyHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	if have, base := historySkipBaseline(path); have || base != 0 {
		t.Errorf("header-only file have=%v base=%v, want false/0", have, base)
	}

	if skipAnomalous(200, 450, false, 0) {
		t.Error("no prior history must never be anomalous")
	}
}

// TestHistorySkipBaselineOldFiveColumnFile pins backward compatibility with an
// existing five-column TEST_HISTORY.md: old short rows (no skipped column) must
// parse with skipped = 0 rather than choke the reader or shift columns.
func TestHistorySkipBaselineOldFiveColumnFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	old := "# Test history — x\n\nbudget_s: 60\n\n| date (UTC) | commit | wall_s | tests | runner |\n|---|---|---|---|---|\n" +
		"| 2026-09-08T23:05Z | fe10852+ | 13.6 | 446 | sadams |\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	rows := parseHistoryRows([]byte(old))
	if len(rows) != 1 {
		t.Fatalf("parseHistoryRows(old 5-col) = %d rows, want 1", len(rows))
	}
	if rows[0].tests != 446 || rows[0].skipped != 0 || rows[0].skippedKnown || rows[0].runner != "sadams" {
		t.Errorf("old row parsed tests=%d skipped=%d known=%v runner=%q, want 446/0/false/sadams", rows[0].tests, rows[0].skipped, rows[0].skippedKnown, rows[0].runner)
	}

	have, base := historySkipBaseline(path)
	if have {
		t.Error("old history reported a skip baseline, but its absent skip count is unknown")
	}
	if base != 0 {
		t.Errorf("baseline from old rows = %v, want 0", base)
	}
}

// TestHistorySkipBaselineNewSixColumn pins that a skipped column written by the
// current tool round-trips: the baseline is the median skipped fraction.
func TestHistorySkipBaselineNewSixColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	new := "# Test history — x\n\nbudget_s: 60\n\n| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|\n" +
		"| 2026-09-08T23:05Z | aa11111 | 13.6 | 446 | 2 | sadams |\n" +
		"| 2026-09-08T23:30Z | bb22222 | 14.8 | 450 | 6 | sadams |\n" +
		"| 2026-09-09T00:10Z | cc33333 | 14.1 | 450 | 4 | sadams |\n"
	if err := os.WriteFile(path, []byte(new), 0o644); err != nil {
		t.Fatal(err)
	}
	have, base := historySkipBaseline(path)
	if !have {
		t.Fatal("new history reported haveHistory=false")
	}
	// Fractions: 2/446=0.00448, 6/450=0.01333, 4/450=0.00889; median = 0.00889.
	want := 4.0 / 450.0
	if base < want-1e-9 || base > want+1e-9 {
		t.Errorf("baseline = %v, want %v (median)", base, want)
	}
}

// TestSkipAnomalous covers the decision rule: a sharp jump above the package's
// own history is anomalous, a normal fraction is not, and no prior history is
// never anomalous.
func TestSkipAnomalous(t *testing.T) {
	// No prior history: never anomalous (a first measurement is accepted).
	if skipAnomalous(200, 450, false, 0) {
		t.Error("no prior history must not be anomalous")
	}
	// A normal small skip fraction against a 0% baseline: not anomalous.
	if skipAnomalous(2, 450, true, 0) {
		t.Error("small skip fraction must not be anomalous")
	}
	// The measured incident: 17.3% versus the honest 8.7% baseline.
	if !skipAnomalous(78, 450, true, 39.0/450.0) {
		t.Error("78/450 skipped vs 39/450 baseline must be anomalous")
	}
	// The measured honest run remains acceptable against its own baseline.
	if skipAnomalous(39, 450, true, 39.0/450.0) {
		t.Error("39/450 skipped vs 39/450 baseline must not be anomalous")
	}
	// A jump measured against an already-elevated baseline.
	if !skipAnomalous(250, 500, true, 0.10) {
		t.Error("50% vs 10% baseline must be anomalous")
	}
	// Ordinary drift within the margin is not anomalous.
	if skipAnomalous(60, 500, true, 0.10) {
		t.Error("12% vs 10% baseline must not be anomalous (within margin)")
	}
}

func TestHistoryWallBaselineAndAnomaly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	history := "# Test history — rules\n\nbudget_s: 15\n\n| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|\n" +
		"| 2026-09-08T22:22Z | aa11111 | 13.5 | 446 | 39 | sadams |\n" +
		"| 2026-09-08T23:09Z | bb22222 | 14.1 | 447 | 39 | sadams |\n" +
		"| 2026-09-09T00:45Z | cc33333 | 14.8 | 450 | 39 | sadams |\n" +
		"| 2026-09-05T19:58Z | old1111 | 20.3 | 208 | 0 | sadams |\n"
	if err := os.WriteFile(path, []byte(history), 0o644); err != nil {
		t.Fatal(err)
	}

	have, baseline := historyWallBaseline(path, 450)
	if !have {
		t.Fatal("comparable real history reported haveHistory=false")
	}
	if wallAnomalous(14.8, 450, have, baseline) {
		t.Error("measured honest 14.8s/450 run must not be anomalous")
	}
	if !wallAnomalous(3.0, 450, have, baseline) {
		t.Error("measured vacuous 3.0s/450 run must be anomalous")
	}
	if have, _ := historyWallBaseline(path, 100); have {
		t.Error("history with no comparable test count must not produce a baseline")
	}
	if wallAnomalous(0.1, 450, false, 1) {
		t.Error("first measurement must not be anomalous")
	}
}

// TestSkipFraction guards the divide-by-zero edge: a run with no tests reported
// has a 0 skip fraction, and a vacuous run never divides by zero.
func TestSkipFraction(t *testing.T) {
	if got := skipFraction(0, 0); got != 0 {
		t.Errorf("skipFraction(0,0) = %v, want 0", got)
	}
	if got := skipFraction(0, 450); got != 0 {
		t.Errorf("skipFraction(0,450) = %v, want 0", got)
	}
	if got := skipFraction(100, 400); got != 0.25 {
		t.Errorf("skipFraction(100,400) = %v, want 0.25", got)
	}
}

// TestMeasurePackageNormalWrites covers the ordinary case: a run with a normal
// skip fraction records its row as usual and contributes exit code 0.
func TestMeasurePackageNormalWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	// A prior history with a ~0% skip baseline (five-column rows, old format).
	old := "# Test history — x\n\nbudget_s: 10\n\n| date (UTC) | commit | wall_s | tests | runner |\n|---|---|---|---|---|\n" +
		"| 2026-09-08T23:05Z | aa11111 | 13.6 | 446 | sadams |\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	res := testResult{elapsed: 8.0, tests: 450, skipped: 2} // 0.4% skipped, under budget
	contrib, wrote := measurePackage("rules", path, "example.com/gorge/rules",
		"2026-09-09T00:05Z", "dd44444", "sadams", res)
	if !wrote {
		t.Error("normal run was not written")
	}
	if contrib != 0 {
		t.Errorf("contrib = %d, want 0", contrib)
	}
	after, _ := os.ReadFile(path)
	if string(after) == string(before) {
		t.Error("normal run did not append a row")
	}
	// The row must carry the skip count.
	if !strings.Contains(string(after), "| 2026-09-09T00:05Z | dd44444 | 8.0 | 450 | 2 | sadams |") {
		t.Errorf("appended row missing skip count\n%s", after)
	}
}

// TestMeasurePackageAnomalousRefuses is the point of the task: a run whose skip
// fraction jumped sharply above the package's own history must write NO row and
// contribute exitInfra (3).
func TestMeasurePackageAnomalousRefuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")
	old := "# Test history — x\n\nbudget_s: 10\n\n| date (UTC) | commit | wall_s | tests | runner |\n|---|---|---|---|---|\n" +
		"| 2026-09-08T23:05Z | aa11111 | 13.6 | 446 | sadams |\n" +
		"| 2026-09-08T23:30Z | bb22222 | 14.8 | 450 | sadams |\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	// 200 of 450 skipped (44%) against a prior history of ~0%: a jagged jump.
	res := testResult{elapsed: 3.6, tests: 450, skipped: 200}
	contrib, wrote := measurePackage("rules", path, "example.com/gorge/rules",
		"2026-09-09T00:05Z", "dd44444", "sadams", res)
	if wrote {
		t.Error("anomalous run was written; it must be refused")
	}
	if contrib != exitInfra {
		t.Errorf("contrib = %d, want exitInfra (%d)", contrib, exitInfra)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Errorf("anomalous run modified the history file (must write no row)\n%s", after)
	}
}

// TestMeasurePackageNoHistoryAccepted pins that a first measurement of a package
// with no prior history is not treated as anomalous, even with a high skip
// fraction: there is nothing to compare against.
func TestMeasurePackageMeasuredRulesCases(t *testing.T) {
	history := "# Test history — rules\n\nbudget_s: 15\n\n| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|\n" +
		"| 2026-09-08T22:22Z | aa11111 | 13.5 | 446 | 38 | sadams |\n" +
		"| 2026-09-08T23:09Z | bb22222 | 14.1 | 447 | 39 | sadams |\n" +
		"| 2026-09-09T00:45Z | cc33333 | 14.8 | 450 | 39 | sadams |\n"

	t.Run("honest run accepted", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "TEST_HISTORY.md")
		if err := os.WriteFile(path, []byte(history), 0o644); err != nil {
			t.Fatal(err)
		}
		contrib, wrote := measurePackage("rules", path, "example.com/rules", "date", "commit", "runner",
			testResult{elapsed: 14.8, tests: 450, skipped: 39})
		if !wrote || contrib != 0 {
			t.Errorf("honest run wrote=%v contrib=%d, want true/0", wrote, contrib)
		}
	})

	t.Run("missing corpus run refused", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "TEST_HISTORY.md")
		if err := os.WriteFile(path, []byte(history), 0o644); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(path)
		contrib, wrote := measurePackage("rules", path, "example.com/rules", "date", "commit", "runner",
			testResult{elapsed: 3.0, tests: 450, skipped: 78})
		if wrote || contrib != exitInfra {
			t.Errorf("vacuous run wrote=%v contrib=%d, want false/%d", wrote, contrib, exitInfra)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Error("refused run modified history")
		}
	})
}

func TestMeasurePackageNoHistoryAccepted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST_HISTORY.md")

	res := testResult{elapsed: 3.6, tests: 450, skipped: 200}
	contrib, wrote := measurePackage("rules", path, "example.com/gorge/rules",
		"2026-09-09T00:05Z", "dd44444", "sadams", res)
	if !wrote {
		t.Error("first measurement with no prior history was refused; it must be recorded")
	}
	if contrib != 0 {
		t.Errorf("contrib = %d, want 0", contrib)
	}
}
