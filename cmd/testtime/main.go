// Command testtime measures how long each Go package's tests take and records
// the result, plus a hard budget, in a per-package TEST_HISTORY.md. It is the
// enforcement point for the repo's test-time budget: a commit cannot silently
// grow test time past a package's budget.
//
// Usage:
//
//	go run ./cmd/testtime [-changed] [-all] [-runner NAME] [pkgs...]
//
//	-changed  measure packages whose staged files include a real change
//	          (a .go file, or any file other than the history files this tool
//	          and allocgate write)
//	-all      measure every package with _test.go files
//	pkgs...   explicit package patterns (e.g. ./host/)
//
// Exactly one selection mode is used; with none, pkgs... is required. The
// selected packages are measured with a single `go test -count=1 -p=1 -json`
// invocation. Each package's wall time comes from the package-level pass/fail
// event's Elapsed field, and the test count is the number of top-level tests.
// The number of top-level tests that SKIPPED is counted too and recorded in the
// row's `skipped` column, so a reader can tell a real measurement from a vacuous
// one. When a run's skip fraction jumps sharply above the package's own history,
// or its wall time per test collapses while the test count remains comparable,
// the row is NOT written and the tool exits with exitInfra (3) -- distinct from
// a budget miss (exitBudget, 1).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adams-shaun/gorge/cards"
)

// Exit codes. exitBudget (1) means a package's wall time exceeded its budget;
// exitInfra (3) means the measurement itself failed (no package produced a
// result), so the hook can tell an infrastructure failure from a budget one.
const (
	exitBudget = 1
	exitInfra  = 3
)

type pkgInfo struct {
	importPath string
	dir        string // relative to cwd
	hasTests   bool
}

type testResult struct {
	elapsed float64
	tests   int
	skipped int
}

func main() {
	changed := flag.Bool("changed", false, "measure packages whose staged files include a real change (a non-history file)")
	all := flag.Bool("all", false, "measure every package with _test.go files")
	runner := flag.String("runner", "", "runner name recorded in TEST_HISTORY.md")
	flag.Parse()
	args := flag.Args()

	cwd, err := os.Getwd()
	if err != nil {
		fatal("getwd: %v", err)
	}

	pkgs := listPackages(cwd)
	byImport := map[string]pkgInfo{}
	for _, p := range pkgs {
		byImport[p.importPath] = p
	}

	var selected []string // import paths
	switch {
	case *changed:
		staged := stagedFiles()
		// The tool's own output (the TEST_HISTORY.md rows it appends) must
		// never feed back into which packages it measures. A measurement that
		// did not land -- a commit that failed after this tool rewrote a
		// package's history file, leaving it staged and dirty -- used to make
		// the next run see every such package as changed and re-measure all of
		// them concurrently, reporting wall times inflated by that load against
		// budgets calibrated on an unloaded box. A package whose only staged
		// change is bookkeeping is NOT a changed package.
		touched := nonArtifacts(staged)
		if len(touched) == 0 {
			if len(staged) > 0 {
				// Wedged state: only the gate's own history files are staged (left
				// over from a commit that failed after they were measured). Measure
				// nothing, and say so plainly instead of silently re-running every
				// package under load.
				fmt.Println("testtime: staged changes are only gate bookkeeping (TEST_HISTORY.md / ALLOC_HISTORY.md), no code changed; nothing measured")
			} else {
				// Empty index: -changed selects packages from the STAGED set, and
				// right after a commit or a rebase there is nothing staged, so the
				// selection is empty. Exit 0 is deliberate -- this is not a failure
				// to measure, it is nothing to measure, and the pre-commit hook never
				// reaches the tool in this state anyway (it filters .go-free commits
				// before calling it, so nothing staged can never reach it). The fix
				// is the stderr message: a human reading a terminal must not be able
				// to mistake an empty selection for a clean measurement.
				fmt.Fprintln(os.Stderr,
					"testtime: -changed selects packages from the staged set (git diff --cached --name-only); nothing is staged, so no packages were measured -- this is an empty selection, not a measurement. Stage the files you changed, or name packages explicitly, e.g. go run ./cmd/testtime ./rules")
			}
			return
		}
		selected = packagesForFiles(touched, pkgs)
		if len(selected) == 0 {
			// Staged files exist but none of them is inside a Go package (a doc
			// change, a web/ file...): the same empty-selection trap as an empty
			// index, and the same answer -- name the packages instead of expecting
			// the staged set to imply them.
			fmt.Fprintln(os.Stderr,
				"testtime: -changed selects packages from the staged set (git diff --cached --name-only); none of the staged files is inside a Go package, so no packages were measured -- this is an empty selection, not a measurement. Name packages explicitly, e.g. go run ./cmd/testtime ./rules")
			return
		}
	case *all:
		for _, p := range pkgs {
			if p.hasTests {
				selected = append(selected, p.importPath)
			}
		}
	default:
		selected = expandArgs(args)
	}
	if len(selected) == 0 {
		return
	}
	sort.Strings(selected)

	results, runErr := runGoTest(selected)
	if len(results) == 0 {
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "testtime: go test failed: %v\n", runErr)
		} else {
			fmt.Fprintf(os.Stderr, "testtime: go test produced no results for %d selected package(s); nothing was measured\n", len(selected))
		}
		os.Exit(exitInfra)
	}

	runnerName := *runner
	if runnerName == "" {
		runnerName = os.Getenv("DS4_AGENT")
	}
	if runnerName == "" {
		runnerName = os.Getenv("USER")
	}
	commit := headCommit()
	date := time.Now().UTC().Format("2006-01-02T15:04Z")

	// A package that did not COMPILE still produces a package-level "fail"
	// event, so results is non-empty and the len(results)==0 branch above
	// never fires; its result is 0 tests in 0.0s, which sails under every
	// budget. That combination used to record a "0 tests" row in
	// TEST_HISTORY.md and let the commit through -- a gate that cannot tell
	// "passed" from "never ran" is not a gate. Tests that ran and FAILED are
	// a different case and still measure fine (they report a real count), so
	// the guard is specifically runErr plus a package that produced no test
	// events at all.
	if runErr != nil {
		for _, imp := range selected {
			if res, ok := results[imp]; ok && res.tests == 0 {
				fmt.Fprintf(os.Stderr, "testtime: %s produced no test events and go test failed (%v) -- almost certainly a build failure; nothing was measured\n", imp, runErr)
				os.Exit(exitInfra)
			}
		}
	}

	exitCode := 0
	anomalous := false
	for _, imp := range selected {
		res, ok := results[imp]
		if !ok {
			continue
		}
		p := byImport[imp]
		path := filepath.Join(p.dir, "TEST_HISTORY.md")
		contrib, _ := measurePackage(p.dir, path, p.importPath, date, commit, runnerName, res)
		if contrib == exitInfra {
			// A measurement refused because its skip fraction jumped is an
			// infrastructure problem, not a budget miss. Keep recording the other
			// packages' (valid) measurements, but exit 3 rather than 0 or 1 so the
			// pre-commit hook treats the whole run as failed-to-measure, not as a
			// budget pass.
			anomalous = true
			continue
		}
		if contrib > exitCode {
			exitCode = contrib
		}
	}
	if anomalous {
		os.Exit(exitInfra)
	}
	os.Exit(exitCode)
}

// measurePackage records one package's measurement, or refuses a likely
// vacuous run. Skip growth and a wall-time-per-test collapse are independent
// signals; either one refuses the row. A refused measurement writes nothing.
func measurePackage(pkg, path, importPath, date, commit, runner string, res testResult) (contrib int, wrote bool) {
	haveSkips, skipBaseline := historySkipBaseline(path)
	haveWall, wallBaseline := historyWallBaseline(path, res.tests)
	skipBad := skipAnomalous(res.skipped, res.tests, haveSkips, skipBaseline)
	wallBad := wallAnomalous(res.elapsed, res.tests, haveWall, wallBaseline)
	if skipBad || wallBad {
		fmt.Fprintf(os.Stderr, "testtime: %s: refusing to record this measurement -- the suite likely did not run its real tests", pkg)
		if skipBad {
			fmt.Fprintf(os.Stderr, "; %d/%d top-level tests skipped (%.1f%%), above this package's prior %.1f%%",
				res.skipped, res.tests, skipFraction(res.skipped, res.tests)*100, skipBaseline*100)
		}
		if wallBad {
			fmt.Fprintf(os.Stderr, "; %.4fs/test is less than half the %.4fs/test median of prior rows with comparable test counts",
				res.elapsed/float64(res.tests), wallBaseline)
		}
		fmt.Fprintln(os.Stderr, " (a missing .cards corpus symlink is one proven cause; a partial build or an environment that could not load fixtures are others)")
		return exitInfra, false
	}
	budget := budgetFor(path, res.elapsed)
	writeHistory(path, importPath, budget, date, commit, res.elapsed, res.tests, res.skipped, runner)
	fmt.Printf("testtime: %s %.1fs %d tests %d skipped budget %ds\n", pkg, res.elapsed, res.tests, res.skipped, budget)
	fmt.Printf("testtime: wrote %s\n", path)
	if res.elapsed > float64(budget) {
		fmt.Fprintf(os.Stderr, "testtime: %s took %.1fs, budget %ds (raise budget_s in %s with a Test-Budget-Approved: trailer)\n",
			pkg, res.elapsed, budget, path)
		return exitBudget, true
	}
	return 0, true
}

// packagesForFiles maps changed file paths to the import paths of the packages
// that contain them. A file whose directory is not a package (no _test.go and
// no .go files, so it is not in pkgs) is skipped. A history file the gate tools
// write is skipped too: it is bookkeeping, never a real change, so it can never
// select a package (defence in depth on top of the nonArtifacts scrub). The
// result is deterministic (sorted by the caller).
func packagesForFiles(files []string, pkgs []pkgInfo) []string {
	byDir := map[string]pkgInfo{}
	for _, p := range pkgs {
		byDir[p.dir] = p
	}
	seen := map[string]bool{}
	var selected []string
	for _, f := range files {
		if isToolArtifact(f) {
			continue
		}
		if p, ok := byDir[filepath.Dir(f)]; ok && !seen[p.importPath] {
			seen[p.importPath] = true
			selected = append(selected, p.importPath)
		}
	}
	return selected
}

// listPackages enumerates every package in the module via `go list ./...`.
func listPackages(cwd string) []pkgInfo {
	out, err := exec.Command("go", "list", "-f",
		"{{.ImportPath}}|{{.Dir}}|{{if or .TestGoFiles .XTestGoFiles}}T{{end}}", "./...").Output()
	if err != nil {
		fatal("go list ./...: %v", err)
	}
	var pkgs []pkgInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		dir, err := filepath.Rel(cwd, parts[1])
		if err != nil {
			dir = parts[1]
		}
		pkgs = append(pkgs, pkgInfo{importPath: parts[0], dir: dir, hasTests: parts[2] == "T"})
	}
	return pkgs
}

// runGit runs a child git process. It runs the child with cards.GitEnv() —
// os.Environ() with every inherited GIT_* variable stripped — so a git the
// tool starts is never quietly redirected at whatever repository the caller
// had checked out or staged.
//
// testtime is invoked by .githooks/pre-commit, and git exports GIT_INDEX_FILE
// — and, in a linked worktree, GIT_DIR — to every hook it runs, as paths that
// name the enclosing repository. In a linked worktree those come through
// absolute, but at a plain checkout top-level GIT_INDEX_FILE is the relative
// ".git/index" and GIT_DIR may be relative too; both resolve against the
// process working directory, which testtime does not control when it measures
// a package from a subdirectory. An inherited relative GIT_* would therefore
// redirect diff --cached, rev-parse and status at a .git that does not exist,
// so the tool's own bookkeeping (staged-file selection, the commit stamp) would
// fail or silently read the wrong repository. Scrub on every child git.
func runGit(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = cards.GitEnv()
	return cmd.Output()
}

// stagedFiles returns every staged file path. Unlike a .go-only scan it is the
// full change set, so a package is "changed" when ANY real file in it is staged
// -- the history-file scrub in nonArtifacts/packagesForFiles is what keeps the
// tool's own bookkeeping from counting as a change.
func stagedFiles() []string {
	out, err := runGit("diff", "--cached", "--name-only")
	if err != nil {
		fatal("git diff --cached: %v", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// isToolArtifact reports whether path is one of the history files the gate
// tools (this one and cmd/allocgate) append to a package when they measure it.
// Those files are the tool's own output, so a package whose ONLY change is one
// of them is not a changed package.
func isToolArtifact(path string) bool {
	base := filepath.Base(path)
	return base == "TEST_HISTORY.md" || base == "ALLOC_HISTORY.md"
}

// nonArtifacts drops the gate tools' own bookkeeping from a staged-file list.
func nonArtifacts(files []string) []string {
	var out []string
	for _, f := range files {
		if !isToolArtifact(f) {
			out = append(out, f)
		}
	}
	return out
}

// expandArgs resolves explicit package patterns (e.g. ./host/, ./...) to
// concrete import paths with a single `go list` invocation.
func expandArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	cmdArgs := append([]string{"list", "-f", "{{.ImportPath}}"}, args...)
	out, err := exec.Command("go", cmdArgs...).Output()
	if err != nil {
		fatal("go list %v: %v", args, err)
	}
	var pkgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs
}

// runGoTest runs a single `go test -count=1 -p=1 -json` over all selected
// packages and returns each package's elapsed time and top-level test count,
// plus the command's error (nil on success). The exit code is otherwise
// irrelevant; we parse the JSON stream.
//
// -p=1 is what makes the measurement honest: without it `go test` defaults -p
// to GOMAXPROCS and runs every selected package in parallel, so a run that
// measures N packages reports each package's wall time inflated by the load of
// the other N-1 (plus whatever else the box has on it) while the budget in the
// package's TEST_HISTORY.md was calibrated on a solo run. Sequencing the
// packages makes a package's measured elapsed the same whether it is measured
// alone or as one of a whole-repo run, so the number a full change set compares
// against its budget is the same number a single-package run would have given.
func runGoTest(pkgs []string) (map[string]testResult, error) {
	cmdArgs := append([]string{"test", "-count=1", "-p=1", "-json"}, pkgs...)
	cmd := exec.Command("go", cmdArgs...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return parseJSON(&buf), err
}

// parseJSON parses a `go test -json` stream. The package-level pass/fail event
// (no Test field) carries Elapsed; top-level tests are run events whose Test
// name has no "/" (subtests are excluded).
func parseJSON(r io.Reader) map[string]testResult {
	results := map[string]testResult{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var ev struct {
			Action  string  `json:"Action"`
			Package string  `json:"Package"`
			Test    string  `json:"Test"`
			Elapsed float64 `json:"Elapsed"`
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Package == "" {
			continue
		}
		if ev.Test == "" && (ev.Action == "pass" || ev.Action == "fail") {
			r := results[ev.Package]
			r.elapsed = ev.Elapsed
			results[ev.Package] = r
		} else if ev.Test != "" && ev.Action == "run" && !strings.Contains(ev.Test, "/") {
			r := results[ev.Package]
			r.tests++
			results[ev.Package] = r
		} else if ev.Test != "" && ev.Action == "skip" && !strings.Contains(ev.Test, "/") {
			r := results[ev.Package]
			r.skipped++
			results[ev.Package] = r
		}
	}
	return results
}

// budgetFor returns the existing budget_s from path, or computes a fresh one
// (ceil(wall*1.25), minimum 5) when the file does not exist.
func budgetFor(path string, wall float64) int {
	if b := loadBudget(path); b >= 0 {
		return b
	}
	b := int(math.Ceil(wall * 1.25))
	if b < 5 {
		b = 5
	}
	return b
}

// loadBudget reads budget_s from an existing TEST_HISTORY.md, or -1 if absent.
func loadBudget(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "budget_s:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "budget_s:"))
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	return -1
}

// writeHistory creates TEST_HISTORY.md (with header, budget and table) if it
// does not exist, otherwise appends one row. Rows are newest last. The row
// records the number of top-level tests that were SKIPPED alongside the run
// count, so a reader can tell a real measurement from a vacuous one.
func writeHistory(path, importPath string, budget int, date, commit string, wall float64, tests, skipped int, runner string) {
	row := fmt.Sprintf("| %s | %s | %.1f | %d | %d | %s |\n", date, commit, wall, tests, skipped, runner)
	if data, err := os.ReadFile(path); err == nil {
		// Existing histories have a five-column header. Upgrade only the header
		// and separator before the first six-column append; historical data rows
		// remain byte-for-byte evidence, while new rows no longer render with a
		// stray cell beyond the table.
		oldHeader := "| date (UTC) | commit | wall_s | tests | runner |\n|---|---|---|---|---|"
		newHeader := "| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|"
		data = bytes.Replace(data, []byte(oldHeader), []byte(newHeader), 1)
		data = append(data, row...)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fatal("write %s: %v", path, err)
		}
		return
	}
	content := fmt.Sprintf("# Test history — %s\n\nbudget_s: %d\n\n| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|\n%s",
		importPath, budget, row)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

// skipAnomalyMargin is the permitted increase over the historical median skip
// fraction. The measured rules runs are 39/450 (8.7%) with the corpus and
// 78/450 (17.3%) without it: an 8.7-point gap. Six points catches that incident
// while allowing up to 27 additional skips in a 450-test suite as ordinary
// test/fixture drift. That allowance comes directly from the measured 450-test
// population; it is not an assertion that the honest baseline is near zero.
const skipAnomalyMargin = 0.06

// Wall-time comparisons use rows whose test count is within 5% of this run,
// avoiding false comparisons across historical suite-size changes. A run below
// half the median wall time per test is refused. This preserves the measured
// honest rules spread (12.0-14.8s at 445-450 tests, under 20% around its median)
// while decisively catching the measured 14.8s -> 3.0s collapse.
const (
	comparableTestMargin = 0.05
	wallCollapseRatio    = 0.50
)

// skipFraction returns the fraction of top-level tests in a run that were
// skipped. A run with no tests reported is treated as 0 (nothing to be
// anomalous about).
func skipFraction(skipped, tests int) float64 {
	if tests == 0 {
		return 0
	}
	return float64(skipped) / float64(tests)
}

// historyRow is one parsed data row of a TEST_HISTORY.md.
type historyRow struct {
	date, commit string
	wall         float64
	tests        int
	skipped      int
	skippedKnown bool
	runner       string
}

// parseHistoryRows parses the data rows of a TEST_HISTORY.md. It accepts both
// the current six-column rows (date, commit, wall_s, tests, skipped, runner)
// and the older five-column rows written before the skipped column existed
// (date, commit, wall_s, tests, runner); an old row is read with skipped = 0.
// Header, separator and prose (budget_s:) lines are ignored.
func parseHistoryRows(data []byte) []historyRow {
	var rows []historyRow
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") {
			continue
		}
		fields := splitRow(line)
		if len(fields) < 5 {
			continue
		}
		// The table header (`| date (UTC) | commit | ...`) is not a data row.
		if fields[0] == "date (UTC)" || fields[1] == "commit" {
			continue
		}
		var r historyRow
		r.date = fields[0]
		r.commit = fields[1]
		r.wall, _ = strconv.ParseFloat(fields[2], 64)
		r.tests, _ = strconv.Atoi(fields[3])
		if len(fields) >= 6 {
			r.skipped, _ = strconv.Atoi(fields[4])
			r.skippedKnown = true
			r.runner = fields[5]
		} else {
			r.runner = fields[4]
		}
		rows = append(rows, r)
	}
	return rows
}

// splitRow splits a `| a | b |` table line into its non-empty, trimmed cells.
func splitRow(line string) []string {
	var out []string
	for _, p := range strings.Split(line, "|") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// historySkipBaseline reads an existing TEST_HISTORY.md and reports whether it
// holds at least one prior measurement with a recorded skip count and the
// median skip fraction those rows record. Old five-column rows parse correctly,
// but their absent skip count is unknown rather than evidence of zero skips, so
// they do not establish a skip baseline. A missing, rowless, or legacy-only file
// means "no prior skip measurement", and the run cannot be refused on skips.
func historySkipBaseline(path string) (haveHistory bool, baseline float64) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0
	}
	rows := parseHistoryRows(data)
	fracs := make([]float64, 0, len(rows))
	for _, r := range rows {
		if r.tests > 0 && r.skippedKnown {
			fracs = append(fracs, float64(r.skipped)/float64(r.tests))
		}
	}
	if len(fracs) == 0 {
		return false, 0
	}
	return true, median(fracs)
}

// skipAnomalous reports whether a run's skip fraction has jumped sharply above
// its package's own history. A package with no prior measurement is never
// anomalous. The median of the prior rows is the baseline so that a single
// historically noisy row cannot mask a genuine jump.
func skipAnomalous(skipped, tests int, haveHistory bool, baseline float64) bool {
	if !haveHistory {
		return false
	}
	return skipFraction(skipped, tests)-baseline > skipAnomalyMargin
}

// historyWallBaseline returns the median wall time per test among usable prior
// rows whose test count is within 5% of the current count. No comparable rows
// means no baseline, so a first measurement (or a substantially resized suite)
// cannot be refused by the wall predicate.
func historyWallBaseline(path string, tests int) (haveHistory bool, baseline float64) {
	if tests <= 0 {
		return false, 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0
	}
	var ratios []float64
	for _, r := range parseHistoryRows(data) {
		if r.tests <= 0 || r.wall <= 0 || math.Abs(float64(r.tests-tests))/float64(tests) > comparableTestMargin {
			continue
		}
		ratios = append(ratios, r.wall/float64(r.tests))
	}
	if len(ratios) == 0 {
		return false, 0
	}
	return true, median(ratios)
}

func wallAnomalous(wall float64, tests int, haveHistory bool, baseline float64) bool {
	if !haveHistory || tests <= 0 {
		return false
	}
	return wall/float64(tests) < baseline*wallCollapseRatio
}

func median(values []float64) float64 {
	sort.Float64s(values)
	return values[len(values)/2]
}

// headCommit returns `git rev-parse --short HEAD`, with a "+" suffix when the
// working tree is dirty.
func headCommit() string {
	out, err := runGit("rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown"
	}
	sha := strings.TrimSpace(string(out))
	status, _ := runGit("status", "--porcelain")
	if len(strings.TrimSpace(string(status))) > 0 {
		sha += "+"
	}
	return sha
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "testtime: "+format+"\n", a...)
	os.Exit(2)
}
