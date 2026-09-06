// Command allocgate runs a package's test binary under pinned GOMAXPROCS and
// GOMEMLIMIT, measures its total allocation and its peak RSS, and fails when
// either exceeds the package's recorded budget.
//
// Why this exists: gorge's gates (build, vet, tests, gcgate) are ratios or
// pass/fail correctness signals. gcgate budgets the CPU *share* the collector
// eats, and a ratio is blind to scale — a change that doubles both real work
// and garbage leaves it flat. allocgate is the absolute counterpart: total
// allocation measures how much GC work the code generates, and peak RSS is
// what actually OOMs a shared box. Pegging both catches a single host test
// that grew to 5.3 GB of allocation with nothing noticing.
//
// # What is measured and how
//
// For each selected package the test binary is compiled with `go test -c`,
// then run with `-test.count=1 -test.memprofile=FILE` under pinned
// GOMAXPROCS and GOMEMLIMIT.
//
//   - Peak RSS comes from the child's getrusage, via
//     cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss, which is
//     kilobytes on Linux. No shelling out to /usr/bin/time.
//   - Total allocation is read from the memory profile the test framework
//     wrote, via `go tool pprof -top -sample_index=alloc_space FILE`. The
//     footer "Showing nodes accounting for X, ... of X total" names the
//     cumulative allocation in the profile. gorge is stdlib-only, so the
//     protobuf is not decoded in-process (github.com/google/pprof is a module
//     dependency the repo forbids); going through `go tool pprof`, which every
//     Go install already carries, adds none.
//
// The memory profile samples allocations at the runtime's default 512 KB
// rate, so the number is an estimate of the true total, not an exact count;
// the 1.25x budget absorbs sampling variance. The unit suffix (B / kB / MB /
// GB / TB / PB) is parsed explicitly and converted to megabytes.
//
// # Budgets
//
// A package's budgets live in its ALLOC_HISTORY.md as alloc_budget_mb and
// rss_budget_mb. They are locked at the first measurement (max(16,
// ceil(measured * 1.25))); later runs reuse them, so the file is a ratchet
// and a run never loosens it. The controller tightens budgets after the
// sibling tasks that are cutting these numbers land.
//
// # Usage
//
//		go run ./cmd/allocgate [-changed] [-all] [-runner NAME]
//			[-maxprocs 32] [-memlimit 5GiB] [pkgs...]
//
//	-changed  measure packages whose staged files include a .go file
//	-all      measure every package with _test.go files
//	pkgs...   explicit package patterns (e.g. ./host/)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// Exit codes mirror cmd/testtime. exitBudget (1) means at least one package
// exceeded its allocation or RSS budget; exitInfra (3) means the measurement
// itself failed, so a hook can distinguish a budget failure from a broken run.
const (
	exitBudget = 1
	exitInfra  = 3
)

type pkgInfo struct {
	importPath string
	dir        string // relative to cwd
	hasTests   bool
}

type measure struct {
	rssMB   float64
	allocMB float64
}

// budgets is the pair binding one package. -1 means "no budget recorded yet",
// in which case the first measurement creates one from today's numbers.
type budgets struct {
	allocMB int
	rssMB   int
}

func main() {
	changed := flag.Bool("changed", false, "measure packages whose staged files include a .go file")
	all := flag.Bool("all", false, "measure every package with _test.go files")
	runner := flag.String("runner", "", "runner name recorded in ALLOC_HISTORY.md")
	procs := flag.Int("maxprocs", 32, "GOMAXPROCS for each run; pinned so the budget is a property of the code, not the box")
	memlimit := flag.String("memlimit", "5GiB", "GOMEMLIMIT for each run (the box's shared-memory cap)")
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
		files := stagedGoFiles()
		if len(files) == 0 {
			return // no staged .go file: skip entirely
		}
		selected = packagesForFiles(files, pkgs)
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

	runnerName := *runner
	if runnerName == "" {
		runnerName = os.Getenv("DS4_AGENT")
	}
	if runnerName == "" {
		runnerName = os.Getenv("USER")
	}
	commit := headCommit()
	date := utcTimestamp()

	exitCode := 0
	for _, imp := range selected {
		p, ok := byImport[imp]
		if !ok {
			continue
		}
		m, err := measureOne(p, *procs, *memlimit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "allocgate: %s: %v\n", p.dir, err)
			exitCode = exitInfra
			continue
		}
		path := filepath.Join(p.dir, "ALLOC_HISTORY.md")
		b := budgetsFor(path, m)
		writeHistory(path, p.importPath, b, date, commit, m, runnerName)
		fmt.Printf("allocgate %s  rss %6.0f MB  alloc %10.0f MB  rss_budget %d MB  alloc_budget %d MB\n",
			p.dir, m.rssMB, m.allocMB, b.rssMB, b.allocMB)
		fmt.Printf("allocgate: wrote %s\n", path)
		if m.rssMB > float64(b.rssMB) {
			fmt.Fprintf(os.Stderr,
				"allocgate: %s peaked at %.0f MB RSS, over the %d MB rss_budget_mb in %s (raise it with the controller)\n",
				p.dir, m.rssMB, b.rssMB, path)
			exitCode = max(exitCode, exitBudget)
		}
		if m.allocMB > float64(b.allocMB) {
			fmt.Fprintf(os.Stderr,
				"allocgate: %s allocated %.0f MB total, over the %d MB alloc_budget_mb in %s (raise it with the controller)\n",
				p.dir, m.allocMB, b.allocMB, path)
			exitCode = max(exitCode, exitBudget)
		}
	}
	os.Exit(exitCode)
}

// measureOne builds pkg's test binary and runs it under the pinned runtime
// caps, returning the peak RSS (MB) and total allocation (MB).
func measureOne(p pkgInfo, procs int, memlimit string) (measure, error) {
	dir, err := os.MkdirTemp("", "allocgate")
	if err != nil {
		return measure{}, err
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "pkg.test")
	build := exec.Command("go", "test", "-c", "-o", bin, p.importPath)
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return measure{}, fmt.Errorf("building test binary: %w", err)
	}

	profile := filepath.Join(dir, "mem.out")
	cmd := exec.Command(bin, "-test.count=1", "-test.memprofile="+profile)
	memLim := memlimit
	cmd.Dir = p.dir // tests that reach for testdata rely on the package directory
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GOMAXPROCS=%d", procs),
		"GOMEMLIMIT="+memLim,
	)
	// A passing suite's own output is just noise for the gate; it is captured
	// so that only a FAILING run surfaces its diagnostics.
	var suiteOut bytes.Buffer
	cmd.Stdout = &suiteOut
	cmd.Stderr = &suiteOut
	runErr := cmd.Run()

	rssKB, ok := peakRSSKB(cmd)
	if !ok || rssKB <= 0 {
		return measure{}, fmt.Errorf("could not read the child's peak RSS from getrusage")
	}

	allocMB, err := totalAllocMB(profile)
	if err != nil {
		return measure{}, err
	}

	if runErr != nil {
		// A failing suite still allocates (the framework writes the profile
		// even on failure), so its numbers are real and worth recording. Whether
		// its tests passed is the build/test gate's business, not this one's;
		// a red page must not take the allocation ratchet offline, because that
		// is how a large allocation regressions slips past a gate mid-flight.
		// Surface the diagnostics so the failure is not silent.
		fmt.Fprintf(os.Stderr, "allocgate: %s: test suite failed, recording its allocation anyway:\n", p.dir)
		fmt.Fprint(os.Stderr, suiteOut.String())
	}
	return measure{rssMB: rssKB / 1024, allocMB: allocMB}, nil
}

// peakRSSKB is the child's peak resident set, from getrusage. Maxrss is in
// kilobytes on Linux. A platform whose ProcessState does not carry a
// *syscall.Rusage is reported rather than silently guessed at: a wrong RSS is
// a lying gate.
func peakRSSKB(cmd *exec.Cmd) (float64, bool) {
	if cmd.ProcessState == nil {
		return 0, false
	}
	ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0, false
	}
	return float64(ru.Maxrss), true
}

// totalAllocMB is the cumulative allocation reported by the memory profile.
// The test mushed its numbers into a gzipped protobuf; gorge is stdlib-only,
// so the bytes are decoded by `go tool pprof`, which every Go install ships,
// rather than by an in-process module dependency. The -top footer names the
// total, and the suffix has to be read carefully because it is
// auto-scaled: a five-gigabyte allocation prints as 4.66GB, not 4770MB.
func totalAllocMB(profile string) (float64, error) {
	out, err := exec.Command("go", "tool", "pprof", "-top", "-sample_index=alloc_space", profile).Output()
	if err != nil {
		return 0, fmt.Errorf("go tool pprof: %w", err)
	}
	s := string(out)
	// Guard the sample index choice: alloc_space (cumulative) is what we
	// budget, and the inuse_space default measures the heap at one instant.
	if !strings.Contains(s, "Type: alloc_space") {
		return 0, fmt.Errorf("profile did not select alloc_space (pprof sample-index mishandled)")
	}
	return parseTotalMB(s)
}

// totalAllocLine matches `Showing nodes accounting for 4.66GB, 100% of 4.66GB
// total`. The "of ... total" figure is the sum of every alloc_space sample,
// i.e. the cumulative allocation across the whole run.
var totalAllocLine = regexp.MustCompile(`of\s+(\d+(?:\.\d+)?)\s*(B|kB|MB|GB|TB|PB)\s+total\b`)

// utcTimestamp stamps a history row with a UTC time like 2026-09-06T17:44Z.
// allocgate is subject to the repository's D16 architecture rule, which
// forbids importing `time` outside host, host/httpapi, cmd/gorged and
// cmd/testtime. allocgate is a build-time measurement tool in exactly the
// class cmd/testtime qualifies for, but it does not get the exemption list
// edited here (that lives in internal/archtest, out of scope), so the stamp
// comes from the `date` binary rather than the time package. A broken or
// absent `date` yields an empty stamp rather than a hard failure.
func utcTimestamp() string {
	out, err := exec.Command("date", "-u", "+%Y-%m-%dT%H:%MZ").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// parseTotalMB extracts the "of N unit total" figure from pprof -top output
// and converts it to megabytes. It is the single unit-handling point in the
// tool: a suffix slip here (reading GB as MB) makes the gate silently 1000x
// too tight, which is why it is mutation-tested in main_test.go.
func parseTotalMB(pprofOut string) (float64, error) {
	m := totalAllocLine.FindStringSubmatch(pprofOut)
	if m == nil {
		return 0, fmt.Errorf("no preceding total found in pprof -top output; check the footer line format")
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("parsing total %q: %w", m[1], err)
	}
	factor := map[string]float64{
		"B":  1,
		"kB": 1 << 10,
		"MB": 1 << 20,
		"GB": 1 << 30,
		"TB": 1 << 40,
		"PB": 1 << 50,
	}[m[2]]
	return v * factor / (1 << 20), nil // to MB
}

// budgetsFor returns the recorded budgets from path, or derives fresh ones
// (max(16, ceil(measured * 1.25)), per budget) when the file does not exist. A
// recorded budget is the ratchet and is never changed by a later run.
func budgetsFor(path string, m measure) budgets {
	if b := loadBudgets(path); b.allocMB >= 0 && b.rssMB >= 0 {
		return b
	}
	return budgets{
		allocMB: budget1_25(m.allocMB),
		rssMB:   budget1_25(m.rssMB),
	}
}

func budget1_25(x float64) int {
	b := int(math.Ceil(x * 1.25))
	// Tiny packages sample a handful of allocations at the runtime's 512 KB
	// profile rate, so a freshly-measured 5 MB can be read as 2 MB on the next
	// run and 8 MB on the one after. A hard floor absorbs that noise so the
	// ratchet does not flake on sampling variance; it is low enough that it
	// never applies where a budget actually matters, and is chosen to be a
	// readable round number.
	if b < 16 {
		b = 16
	}
	return b
}

// loadBudgets reads alloc_budget_mb and rss_budget_mb from an existing
// ALLOC_HISTORY.md, or -1 for either when absent.
func loadBudgets(path string) budgets {
	b := budgets{allocMB: -1, rssMB: -1}
	data, err := os.ReadFile(path)
	if err != nil {
		return b
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "alloc_budget_mb:"):
			b.allocMB = parseBudget(strings.TrimPrefix(line, "alloc_budget_mb:"))
		case strings.HasPrefix(line, "rss_budget_mb:"):
			b.rssMB = parseBudget(strings.TrimPrefix(line, "rss_budget_mb:"))
		}
	}
	return b
}

func parseBudget(value string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return n
	}
	return -1
}

// writeHistory creates ALLOC_HISTORY.md (header, both budgets and table) if it
// does not exist, otherwise appends one row. Rows are newest last.
func writeHistory(path, importPath string, b budgets, date, commit string, m measure, runner string) {
	row := fmt.Sprintf("| %s | %s | %.0f | %.0f | %s |\n", date, commit, m.rssMB, m.allocMB, runner)
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
	content := fmt.Sprintf("# Allocation history — %s\n\nalloc_budget_mb: %d\nrss_budget_mb: %d\n\n| date (UTC) | commit | rss_mb | alloc_mb | runner |\n|---|---|---|---|---|\n%s",
		importPath, b.allocMB, b.rssMB, row)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

// packagesForFiles maps staged file paths to the import paths of the packages
// that contain them. A file whose directory is not a package (no _test.go and
// no .go files) is skipped. The result is deterministic (sorted by the caller).
func packagesForFiles(files []string, pkgs []pkgInfo) []string {
	byDir := map[string]pkgInfo{}
	for _, p := range pkgs {
		byDir[p.dir] = p
	}
	seen := map[string]bool{}
	var selected []string
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
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

// stagedGoFiles returns the staged file paths that end in .go.
func stagedGoFiles() []string {
	out, err := exec.Command("git", "diff", "--cached", "--name-only").Output()
	if err != nil {
		fatal("git diff --cached: %v", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasSuffix(line, ".go") {
			files = append(files, line)
		}
	}
	return files
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
	fmt.Fprintf(os.Stderr, "allocgate: "+format+"\n", a...)
	os.Exit(2)
}
