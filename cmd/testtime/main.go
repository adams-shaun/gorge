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
			}
			return
		}
		selected = packagesForFiles(touched, pkgs)
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
	for _, imp := range selected {
		res, ok := results[imp]
		if !ok {
			continue
		}
		p := byImport[imp]
		path := filepath.Join(p.dir, "TEST_HISTORY.md")
		budget := budgetFor(path, res.elapsed)
		writeHistory(path, p.importPath, budget, date, commit, res.elapsed, res.tests, runnerName)
		fmt.Printf("testtime: %s %.1fs %d tests budget %ds\n", p.dir, res.elapsed, res.tests, budget)
		fmt.Printf("testtime: wrote %s\n", path)
		if res.elapsed > float64(budget) {
			fmt.Fprintf(os.Stderr, "testtime: %s took %.1fs, budget %ds (raise budget_s in %s with a Test-Budget-Approved: trailer)\n",
				p.dir, res.elapsed, budget, path)
			exitCode = exitBudget
		}
	}
	os.Exit(exitCode)
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

// stagedFiles returns every staged file path. Unlike a .go-only scan it is the
// full change set, so a package is "changed" when ANY real file in it is staged
// -- the history-file scrub in nonArtifacts/packagesForFiles is what keeps the
// tool's own bookkeeping from counting as a change.
func stagedFiles() []string {
	out, err := exec.Command("git", "diff", "--cached", "--name-only").Output()
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
// does not exist, otherwise appends one row. Rows are newest last.
func writeHistory(path, importPath string, budget int, date, commit string, wall float64, tests int, runner string) {
	row := fmt.Sprintf("| %s | %s | %.1f | %d | %s |\n", date, commit, wall, tests, runner)
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fatal("open %s: %v", path, err)
		}
		defer f.Close()
		if _, err := f.WriteString(row); err != nil {
			fatal("write %s: %v", path, err)
		}
		return
	}
	content := fmt.Sprintf("# Test history — %s\n\nbudget_s: %d\n\n| date (UTC) | commit | wall_s | tests | runner |\n|---|---|---|---|---|\n%s",
		importPath, budget, row)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

// headCommit returns `git rev-parse --short HEAD`, with a "+" suffix when the
// working tree is dirty.
func headCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	sha := strings.TrimSpace(string(out))
	status, _ := exec.Command("git", "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(status))) > 0 {
		sha += "+"
	}
	return sha
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "testtime: "+format+"\n", a...)
	os.Exit(2)
}
