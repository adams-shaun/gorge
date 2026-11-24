package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// scenarioRunFixture is a scripted start on the FDN table: Llanowar Elves is
// the focus seat's only mana source, with Giant Growth in hand, so a spell the
// seat casts is paid by the Elves' mana ability (f0/a0, credited when the
// payer taps the source: design D9).
const scenarioRunFixture = `{"format":"gorge-start-1","focus":"Llanowar Elves","keys":["f0/a0"],"players":2,"opponent":"passive","active":0,
 "setup":{"p0":{"hand":["Giant Growth","Giant Growth"],"battlefield":["Llanowar Elves"]},
          "p1":{"battlefield":["Savannah Lions","Plains"],"hand":["Plains"]}}}`

// scenarioCoverageRows is a .coverage file's rows by card: cast, land, keys used.
func scenarioCoverageRows(t *testing.T, path string) map[string][3]int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string][3]int{}
	for i, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if i == 0 {
			continue
		}
		f := strings.Split(l, "\t")
		var r [3]int
		for j := range r {
			if r[j], err = strconv.Atoi(f[1+j]); err != nil {
				t.Fatalf("%s: row %q: %v", path, l, err)
			}
		}
		rows[f[0]] = r
	}
	return rows
}

// TestScenarioRun plays a scenario directory through -scenario: every file is
// a run named sc-<file> with its .verdict, .coverage, .branches and .evidence
// files; the games are clean; a scripted game credits key uses and never a
// cast or a land play; and a game that ends at decision 0 credits nothing,
// so the setup itself credits nothing (design section 8, "coverage leakage").
func TestScenarioRun(t *testing.T) {
	cardsettest.Require(t, "fdn")
	if _, err := cardset.CorpusDir(); err != nil {
		t.Skip(err)
	}
	oldCov, oldEv, oldBr := coveragePath, evidencePath, branchesPath
	t.Cleanup(func() { coveragePath, evidencePath, branchesPath = oldCov, oldEv, oldBr })
	scen := t.TempDir()
	if err := os.WriteFile(filepath.Join(scen, "llanowar-elves.1.json"), []byte(scenarioRunFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scen, "notes.txt"), []byte("not a scenario"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := func(dir string, more ...string) []string {
		return append([]string{"-scenario", scen, "-set", "fdn", "-pairs", "4", "-seed", "1",
			"-out", dir, "-coverage", dir, "-evidence", dir}, more...)
	}
	// -branches is registered on the process flag set, which runKFuzz replaces:
	// the test sets its value as the flag would.
	withBranches := func(dir string) { branchesPath = dir }

	t.Run("clean games and their files", func(t *testing.T) {
		dir := t.TempDir()
		withBranches(dir)
		code, stdout, stderr := runKFuzz(t, args(dir)...)
		if code != 0 {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
		}
		const name = "sc-llanowar-elves.1"
		verdict, err := os.ReadFile(filepath.Join(dir, name+".verdict"))
		if err != nil {
			t.Fatal(err)
		}
		if string(verdict) != stdout {
			t.Errorf(".verdict is not the run's report:\n%s\nstdout:\n%s", verdict, stdout)
		}
		if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `: 4 games \(0 refused-card, 0 refused-pair, 0 error\), 4 clean, 0 divergent`).Match(verdict) {
			t.Fatalf("verdict line:\n%s", verdict)
		}
		for _, kind := range []string{"coverage", "branches", "evidence"} {
			if _, err := os.Stat(filepath.Join(dir, name+"."+kind)); err != nil {
				t.Errorf("no .%s file: %v", kind, err)
			}
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 4 {
			t.Errorf("%d files in -out, want the four of one run", len(entries))
		}
		rows := scenarioCoverageRows(t, filepath.Join(dir, name+".coverage"))
		for card, r := range rows {
			if r[0] != 0 || r[1] != 0 {
				t.Errorf("%s: %d casts, %d land plays: a scripted game credits neither", card, r[0], r[1])
			}
		}
		// the precondition of the rule above: the games used the focus card's
		// key, so the zeros are the rule's, not an idle game's
		if rows["Llanowar Elves"][2] == 0 {
			t.Errorf("Llanowar Elves: no key used in 4 games (rows %v)", rows)
		}
		if ev, _ := os.ReadFile(filepath.Join(dir, name+".evidence")); len(ev) != 0 {
			t.Errorf("a scripted game credited plays: %q", ev)
		}
	})

	t.Run("a game that ends at decision 0 credits nothing", func(t *testing.T) {
		dir := t.TempDir()
		withBranches("")
		code, stdout, stderr := runKFuzz(t, args(dir, "-max-decisions", "1")...)
		if code != 0 {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
		}
		rows := scenarioCoverageRows(t, filepath.Join(dir, "sc-llanowar-elves.1.coverage"))
		if len(rows) == 0 {
			t.Fatal("no coverage rows: the games were not covered")
		}
		for card, r := range rows {
			if r != [3]int{} {
				t.Errorf("%s: cast %d, land %d, keys used %d after a game that ended at the first decision", card, r[0], r[1], r[2])
			}
		}
		verdict, _ := os.ReadFile(filepath.Join(dir, "sc-llanowar-elves.1.verdict"))
		if !strings.Contains(string(verdict), "4 clean") {
			t.Errorf("games did not end clean at the cap:\n%s", verdict)
		}
	})

	t.Run("a refused scenario is an error, not a game", func(t *testing.T) {
		bad := t.TempDir()
		if err := os.WriteFile(filepath.Join(bad, "nobody.1.json"), []byte(strings.Replace(scenarioRunFixture, "Llanowar Elves", "No Such Card", -1)), 0o644); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		code, stdout, _ := runKFuzz(t, "-scenario", bad, "-set", "fdn", "-pairs", "2", "-out", dir, "-absolute")
		verdict, _ := os.ReadFile(filepath.Join(dir, "sc-nobody.1.verdict"))
		if code == 0 || !strings.Contains(string(verdict), "No Such Card") || !strings.Contains(stdout, "0 clean") {
			t.Errorf("exit %d; verdict:\n%s\nstdout:\n%s", code, verdict, stdout)
		}
	})

	t.Run("usage", func(t *testing.T) {
		for _, a := range [][]string{{"-scenario", scen, "-seats", "3"}, {"-scenario", scen, "-repro", "x"}, {"-scenario", scen, "-synth-with", "tail-52"}} {
			if code, _, _ := runKFuzz(t, a...); code != 2 {
				t.Errorf("%v: exit %d, want 2", a, code)
			}
		}
		if code, _, _ := runKFuzz(t, "-scenario", filepath.Join(scen, "missing")); code != 1 {
			t.Errorf("missing path: exit %d, want 1", code)
		}
	})
}
