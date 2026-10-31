package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

const module = "github.com/adams-shaun/gorge"

type pkg struct {
	path    string
	dir     string          // absolute package directory (go list .Dir)
	imports map[string]bool // direct, non-test
	deps    map[string]bool // transitive, non-test
}

// packages lists every package in the module with its direct and transitive
// non-test imports. Test files are excluded on purpose: tests may import
// anything (view's tests import rules; cards' tests import time).
//
// It also opens every package directory from the test process. go test's
// result cache tracks only the files the test process itself opens
// (os.Open/os.Stat, via internal/testlog) — never the files a `go list`/
// `go doc`/`go build` subprocess reads — and a directory open hashes the
// directory's entry names and their size/mtime (cmd/go/internal/test/test.go
// hashOpen). Opening every package dir therefore makes the go-list-based
// checks cache-correct: adding, removing or editing any module source file
// invalidates this package's cached test result instead of being served a
// stale pass. The subprocess checks that do not otherwise need the package
// map (TestNoExportLeaksAnEngineGame's `go doc`, TestEngineCompilesFor32Bit's
// `go build`) call openModulePackageDirs for the same side effect.
func packages(t *testing.T) map[string]pkg {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}|{{join .Deps " "}}|{{.Dir}}`, module+"/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	pkgs := map[string]pkg{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			t.Fatalf("unexpected go list line %q", line)
		}
		p := pkg{path: parts[0], imports: set(parts[1]), deps: set(parts[2]), dir: parts[3]}
		pkgs[p.path] = p
	}
	if len(pkgs) < 10 {
		t.Fatalf("go list found only %d packages", len(pkgs))
	}
	openModulePackageDirs(t, pkgs)
	return pkgs
}

// openModulePackageDirs opens every package directory in pkgs from the test
// process so go test's result cache tracks the module's source files (see
// packages' doc comment). The directory's entries and their size/mtime are
// what hashOpen folds into the cache key, so the open is enough — the files
// need not be read. It runs once per test binary: the test log is shared by
// every test in the run, so the first open covers them all, and a test
// selected with -run still triggers it because it goes through packages().
// A package with no directory, or a directory that cannot be opened, is
// fatal: silently skipping it would reintroduce the stale-cache hole this
// open closes.
//
// The number of directories opened is recorded in openModuleDirsN so
// TestOpenPackageDirsCoversEveryPackage can assert that packages() actually
// performed the open — removing the openModulePackageDirs call from
// packages() leaves the count at zero and fails that test, instead of
// silently reintroducing the stale-cache hole.
func openModulePackageDirs(t *testing.T, pkgs map[string]pkg) {
	t.Helper()
	openModuleDirsOnce.Do(func() {
		openModuleDirsN, openModuleDirsErr = openPackageDirs(pkgs)
	})
	if openModuleDirsErr != nil {
		t.Fatal(openModuleDirsErr)
	}
}

var (
	openModuleDirsOnce sync.Once
	openModuleDirsErr  error
	openModuleDirsN    int // directories openModulePackageDirs opened through packages()
)

// openPackageDirs opens the directory of every package in pkgs and returns
// how many it opened. It returns an error instead of calling t.Fatal so the
// guards can be unit tested (TestOpenPackageDirsCoversEveryPackage): a caller
// that opens fewer directories than there are packages has silently left the
// stale-cache hole open. The paths are iterated in sorted order so the first
// failure reported (and the "opened" count in its message) is stable across
// runs.
func openPackageDirs(pkgs map[string]pkg) (int, error) {
	if len(pkgs) == 0 {
		return 0, fmt.Errorf("no packages to open; the cache-tracking open would run vacuously")
	}
	paths := make([]string, 0, len(pkgs))
	for path := range pkgs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	opened := 0
	for _, path := range paths {
		p := pkgs[path]
		if p.dir == "" {
			return opened, fmt.Errorf("go list reported no directory for %s; the cache-tracking open would be skipped", path)
		}
		if _, err := os.ReadDir(p.dir); err != nil {
			return opened, fmt.Errorf("open package dir %s (go test's cache must track it): %w", p.dir, err)
		}
		opened++
	}
	return opened, nil
}

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}

// TestTimeIsImportedOnlyByTheHost is spec D16: the host's injected sleep,
// the SSE writer's ticker/keep-alive and gorged's shutdown timeout are the
// only clocks in the system. Every other package must be a pure function
// of its inputs.
//
// The other allowed packages are developer tools that read the clock only to
// bound or report their own run, never anything a game, event, view or replay
// depends on. cmd/cardfuzz reads it only for its per-game hang watchdog and its games/s
// progress line; every deck and game is a pure function of its seed. The
// paymirror A/B harness reads the clock only in its cmd/paymirror CLI: the
// per-game wall-time budget is injected to the library as a
// paymirror.DriverOptions.BudgetExceeded predicate the CLI builds from
// time.Now, and the CLI's progress line prints per-game elapsed seconds. The
// testkit/paymirror library itself imports no time and is a pure function of
// its seeds and the corpus for a completed game; a game the CLI's budget
// truncated says so (Err "truncated: budget"). cmd/enginebench (the docs/015
// engine-speed rows) reads it only to report each row's elapsed wall time
// beside its CPU time; every game and seed is fixed by its flags.
// server/host/manabrewhttp uses time only for SSE keep-alive and write
// deadlines. No game, event, view or replay reads the clock, and intents reach
// the engine only through SubmitIntent.
//
// draft/eventhost reads the clock only for lobby, idle and pick timers, through
// Options.Now/Options.Clock; the event log records their outcome as entries, and
// replay never reads the clock.
func TestTimeIsImportedOnlyByTheHost(t *testing.T) {
	allowed := map[string]bool{
		module + "/server/host":              true,
		module + "/server/host/httpapi":      true,
		module + "/server/gorged":            true,
		module + "/cmd/cardfuzz":             true,
		module + "/cmd/paymirror":            true,
		module + "/cmd/enginebench":          true,
		module + "/server/host/manabrewhttp": true,
		module + "/draft/eventhost":          true,
	}
	for path, p := range packages(t) {
		if p.imports["time"] && !allowed[path] {
			t.Errorf("%s imports time; only the packages in this test's allowed map may (see its doc comment)", path)
		}
	}
}

// TestDependencyOrderHolds pins the arrows that must never appear, direct or
// transitive.
func TestDependencyOrderHolds(t *testing.T) {
	pkgs := packages(t)
	forbidden := []struct{ from, to string }{
		{module + "/engine/effects", module + "/engine/rules"},
		{module + "/engine/view", module + "/engine/rules"},
		{module + "/bots/botpolicy", module + "/engine/view"},
		{module + "/bots/botpolicy", module + "/engine/rules"},
		{module + "/bots/botpolicy", module + "/engine/seat"},
		{module + "/server/protocol", module + "/engine/rules"},
		{module + "/server/protocol", module + "/bots"},
		{module + "/server/host", module + "/testkit/testutil"},
		{module + "/server/host/httpapi", module + "/testkit/testutil"},
		{module + "/server/gorged", module + "/testkit/testutil"},
		{module + "/engine/cards", module + "/engine/state"},
		{module + "/engine/deck", module + "/engine/rules"},
		// The shadow answer (HD-3, human-divergence design §5.4) is computed by
		// a package the host calls and the offline triage tool recomputes
		// through: it may not depend on the host that calls it, nor on test
		// scaffolding.
		{module + "/server/divergence/shadow", module + "/server/host"},
		{module + "/server/divergence/shadow", module + "/testkit/testutil"},
		// The lasagna layering (spec 2026-10-03 §3): engine/rules/cost is the L2
		// cost vocabulary leaf. It sits below effects and below every rules
		// subsystem package, so none of them -- and never rules itself -- may
		// be reachable from it. TestCostVocabularyIsALeaf pins its whole
		// direct import set; these rows name the edges the layering forbids.
		{module + "/engine/rules/cost", module + "/engine/rules"},
		{module + "/engine/rules/cost", module + "/engine/effects"},
		{module + "/engine/rules/cost", module + "/engine/events"},
		{module + "/engine/rules/cost", module + "/engine/decision"},
		{module + "/engine/rules/cost", module + "/bots/botpolicy"},
		{module + "/engine/rules/cost", module + "/engine/rules/pay"},
		{module + "/engine/rules/cost", module + "/engine/rules/chars"},
		{module + "/engine/rules/cost", module + "/engine/rules/trigmatch"},
		{module + "/engine/rules/cost", module + "/engine/rules/combat"},
		{module + "/engine/rules/cost", module + "/engine/rules/resolve"},
		// engine/rules/combat is an L5 subsystem package (W5 step E5): attack and
		// block legality behind combat.Board. It never reaches back into
		// rules (Go forbids the direct cycle once rules imports it; these
		// rows also forbid the transitive one), never into a sibling
		// subsystem package or the resolution kernel above it, and never
		// into the bot layer. (decision is reachable through effects, so it
		// is not a transitive row; TestCombatImportsStayBelowRules pins the
		// direct import set, which keeps decision -- and with it the asks
		// and validators of the declaration flow -- out of the package.)
		{module + "/engine/rules/combat", module + "/engine/rules"},
		{module + "/engine/rules/combat", module + "/engine/rules/pay"},
		{module + "/engine/rules/combat", module + "/engine/rules/trigmatch"},
		{module + "/engine/rules/combat", module + "/engine/rules/resolve"},
		{module + "/engine/rules/combat", module + "/bots/botpolicy"},
		// engine/rules/trigmatch (W5 E3) is an L5 subsystem package: the trigger
		// matchers read the engine through trigmatch.Board, never through
		// *rules.Engine, so it may not reach rules, its sibling L5 packages
		// (pay, combat) or the L6 resolution kernel. It may sit on engine/rules/chars
		// (L4) and engine/rules/cost (L2). TestTrigmatchImportsStayBelowL5 pins its
		// direct import set.
		{module + "/engine/rules/trigmatch", module + "/engine/rules"},
		{module + "/engine/rules/trigmatch", module + "/engine/rules/pay"},
		{module + "/engine/rules/trigmatch", module + "/engine/rules/combat"},
		{module + "/engine/rules/trigmatch", module + "/engine/rules/resolve"},
		// engine/rules/scriptfacts is the pure card-fact leaf of the rules-split plan
		// (2026-10-06): facts a card's printed text and compiled script imply,
		// with no *rules.Engine and no game state. It sits below every rules
		// subsystem, so it may never reach rules or any package above it.
		// TestScriptFactsImportsStayBelowRules pins its whole direct import set;
		// these rows name the edges the layering forbids transitively.
		{module + "/engine/rules/scriptfacts", module + "/engine/rules"},
		{module + "/engine/rules/scriptfacts", module + "/engine/effects"},
		{module + "/engine/rules/scriptfacts", module + "/engine/events"},
		{module + "/engine/rules/scriptfacts", module + "/engine/decision"},
		{module + "/engine/rules/scriptfacts", module + "/bots/botpolicy"},
		{module + "/engine/rules/scriptfacts", module + "/engine/rules/pay"},
		{module + "/engine/rules/scriptfacts", module + "/engine/rules/chars"},
		{module + "/engine/rules/scriptfacts", module + "/engine/rules/trigmatch"},
		{module + "/engine/rules/scriptfacts", module + "/engine/rules/combat"},
		{module + "/engine/rules/scriptfacts", module + "/engine/rules/resolve"},
	}
	for path, p := range pkgs {
		if strings.HasPrefix(path, module+"/bots") {
			for _, target := range []string{module + "/server/host", module + "/testkit/testutil"} {
				if p.deps[target] {
					t.Errorf("%s depends on %s (transitively); bots packages may not", path, target)
				}
			}
		}
	}
	for _, f := range forbidden {
		p, ok := pkgs[f.from]
		if !ok {
			continue // not built yet; the constraint binds once it is
		}
		if p.deps[f.to] {
			t.Errorf("%s depends on %s (transitively); the dependency order forbids it", f.from, f.to)
		}
	}

	for _, msg := range manabrewBoundaryViolations(pkgs) {
		t.Error(msg)
	}
}

// manabrewBoundaryViolations returns one message per ManaBrew dependency
// boundary violation, for the synthetic or live package map pkgs.
//
// The from-manabrew rows check DIRECT imports only, deliberately. The
// translator is allowed to import view, decision and state (spec §5.1), and
// view imports events; the transport is allowed to import host, and host
// imports rules. A transitive check on those rows would therefore forbid the
// imports §5.1 requires, and note that server/host/manabrewhttp cannot avoid host.
// What the boundary actually forbids is the adapter reaching an engine
// package ITSELF — a direct edge — which is what p.imports reports. The
// reverse rows stay transitive: nothing on the engine side may depend on the
// adapter even indirectly.
//
// A row whose from package is absent from pkgs is skipped: these package
// paths are intentionally listed before those packages exist, and the checks
// bind automatically as go list begins reporting each package.
//
// The returned slice is sorted so the helper is deterministic.
func manabrewBoundaryViolations(pkgs map[string]pkg) []string {
	forbiddenManaBrewDirect := []struct{ from, to string }{
		{module + "/server/manabrew", module + "/engine/rules"},
		{module + "/server/manabrew", module + "/engine/effects"},
		{module + "/server/manabrew", module + "/engine/events"},
		{module + "/server/manabrew", module + "/server/host"},
		{module + "/server/manabrew", module + "/server/host/httpapi"},
		{module + "/server/manabrew", module + "/bots/azmcts"},
		{module + "/server/manabrew", module + "/testkit/testutil"},
		{module + "/server/host/manabrewhttp", module + "/engine/rules"},
		{module + "/server/host/manabrewhttp", module + "/engine/effects"},
		{module + "/server/host/manabrewhttp", module + "/testkit/testutil"},
		{module + "/server/host/manabrewhttp", module + "/bots/azmcts"},
	}
	var msgs []string
	for _, f := range forbiddenManaBrewDirect {
		p, ok := pkgs[f.from]
		if !ok {
			continue
		}
		if p.imports[f.to] {
			msgs = append(msgs, fmt.Sprintf("%s imports %s; the ManaBrew dependency boundary forbids it", f.from, f.to))
		}
	}
	var forbiddenManaBrewReverse []struct{ from, to string }
	for _, from := range []string{
		"engine/cards", "engine/state", "engine/decision", "engine/events", "engine/effects", "bots/botpolicy",
		"engine/rules", "engine/view", "engine/seat", "engine/replay", "server/protocol", "server/host", "server/host/httpapi",
	} {
		for _, to := range []string{"server/protocol/manabrew", "server/manabrew", "server/host/manabrewhttp"} {
			forbiddenManaBrewReverse = append(forbiddenManaBrewReverse, struct{ from, to string }{module + "/" + from, module + "/" + to})
		}
	}
	for _, f := range forbiddenManaBrewReverse {
		p, ok := pkgs[f.from]
		if !ok {
			continue
		}
		if p.deps[f.to] {
			msgs = append(msgs, fmt.Sprintf("%s depends on %s (transitively); the ManaBrew dependency boundary forbids it", f.from, f.to))
		}
	}
	sort.Strings(msgs)
	return msgs
}

// TestManaBrewWireIsStdlibOnly keeps the protocol's public wire types free of
// gorge package dependencies. The row is in place before server/protocol/manabrew
// exists and starts enforcing the boundary as soon as it is added.
func TestManaBrewWireIsStdlibOnly(t *testing.T) {
	const wire = module + "/server/protocol/manabrew"
	p, ok := packages(t)[wire]
	if !ok {
		t.Skip("server/protocol/manabrew is not built yet")
	}
	for imp := range p.imports {
		out, err := exec.Command("go", "list", "-f", "{{.Standard}}", imp).Output()
		if err != nil {
			t.Errorf("inspect %s import %s: %v", wire, imp, err)
			continue
		}
		if strings.TrimSpace(string(out)) != "true" {
			t.Errorf("%s imports %s; ManaBrew wire types must be stdlib-only", wire, imp)
		}
	}
}

// TestNoExportLeaksAnEngineGame is D6's compile-time half: no exported
// function or method of host or server/host/httpapi may expose a *state.Game or a
// *rules.Engine through its signature — by return type, or by any parameter
// shape a caller could funnel back into a leaked live game. Every value that
// crosses either package is already a view.View, a protocol.* or a
// decision.* payload; the engine and its game never leave engine/rules/ at all (a
// client layer must read state only through view).
//
// The scan is deliberately structural and text-restricted: it reads `go doc
// -all` output and considers ONLY the `func` declaration lines — the
// signatures — never the doc prose. A bare-substring sweep over the whole
// doc block (the plan's sketch) would false-positive on the Events method's
// own doc, which legitimately says "state.Game.Clone" to explain what it
// copies; matching only the declaration lines keeps that prose out of scope.
// The token is boundary-matched so an identifier that merely STARTS with one
// of these type names (were state.GameX or rules.EngineY ever to exist)
// still does not trip it.
func TestNoExportLeaksAnEngineGame(t *testing.T) {
	// `go doc` reads host's source through a subprocess, invisible to go
	// test's result cache. packages() opens every module package directory
	// so a source change invalidates this test's cached result (see its doc
	// comment); the returned map is unused here.
	packages(t)
	// These exact type names (state.Game, rules.Engine) are what a leak's
	// signature would carry; a legitimate client-facing type never has an
	// element in either package. Word boundaries stop the match from
	// prefix-colliding with a hypothetical longer identifier.
	leaks := []*regexp.Regexp{
		regexp.MustCompile(`\bstate\.Game\b`),
		regexp.MustCompile(`\brules\.Engine\b`),
	}
	for _, pkg := range []string{module + "/server/host", module + "/server/host/httpapi"} {
		out, err := exec.Command("go", "doc", "-all", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go doc -all %s: %v", pkg, err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, "func ") {
				continue // signatures only; prose that merely names the type is not a leak
			}
			for _, re := range leaks {
				if re.MatchString(line) {
					t.Errorf("%s exposes an engine game through %q", pkg, line)
				}
			}
		}
	}
}

// TestNoLegacyMathRand: math/rand/v2 with an explicit seeded source is the
// only randomness (engine/rules/rng.go, bots/heuristic/bot.go). The v1 package's global
// functions are exactly the ambient randomness the engine spec forbids.
func TestNoLegacyMathRand(t *testing.T) {
	for path, p := range packages(t) {
		if p.imports["math/rand"] {
			t.Errorf("%s imports math/rand; use math/rand/v2 with a seeded source", path)
		}
	}
}

// resumeFieldWriters walks the non-test engine/rules/ sources (recursively, so a
// engine/rules/* subpackage stays in the census) and returns every
// function that assigns to the Engine.resume state, keyed by the receiver-
// qualified function name (e.g. "(*Engine).Ask") with the sites it writes.
// A write reachable through the resume field is a write to the resume state:
// e.resume, e.resume.outer and e.resume.outer.sa all count (fx40), because
// resolution.go carries a real nested write of the shape e.resume.outer = ...
// that the pre-fx40 outermost-selector-only match never saw. It is the census
// that TestResumeStateOwnedOnlyByTheResolutionMachinery both enforces against
// and keeps exact.
func resumeFieldWriters(t *testing.T) map[string][]string {
	t.Helper()
	dir := filepath.Join("..", "..", "engine", "rules")
	files := goSourcesUnder(t, dir)
	if len(files) == 0 {
		t.Fatalf("no rules sources found under %s (cwd %s)", dir, mustCwd())
	}
	out := map[string][]string{}
	for _, file := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		// The enclosing function for a position is the func span of smallest
		// extent that contains it — the innermost, whether named or a func
		// literal. Named methods are what the allow-list keys on.
		type span struct {
			name string
			pos  token.Pos
			end  token.Pos
		}
		var spans []span
		ast.Inspect(f, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				spans = append(spans, span{funcQualName(fd), fd.Pos(), fd.End()})
			}
			return true
		})
		enclosing := func(p token.Pos) string {
			best := ""
			bestSpan := token.Pos(1 << 30)
			for _, s := range spans {
				if s.pos <= p && p <= s.end && s.end-s.pos < bestSpan {
					best = s.name
					bestSpan = s.end - s.pos
				}
			}
			return best
		}
		rel, err := filepath.Rel(filepath.Join("..", ".."), file)
		if err != nil {
			t.Fatalf("rel %s: %v", file, err)
		}
		rel = filepath.ToSlash(rel)
		ast.Inspect(f, func(n ast.Node) bool {
			check := func(target ast.Expr) {
				resumeSel, ok := resumeSelectorInChain(target)
				if !ok {
					return
				}
				fn := enclosing(resumeSel.Pos())
				out[fn] = append(out[fn], fmt.Sprintf("%s:%d", rel, fset.Position(resumeSel.Pos()).Line))
			}
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					check(lhs)
				}
			case *ast.IncDecStmt:
				check(node.X)
			}
			return true
		})
	}
	return out
}

// goSourcesUnder returns every non-test .go file under dir, RECURSIVELY
// (testdata and dot/underscore directories excluded), sorted. A census that
// globbed only dir/*.go would silently drop every file a refactor moves into
// a subpackage (the rules-engine refactor spec's W5 package extraction).
func goSourcesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(files)
	return files
}

// resumeSelectorInChain reports whether any selector in an assignment target's
// selector chain is named "resume", and if so returns that selector. It is
// what makes e.resume, e.resume.outer and e.resume.outer.sa all writes to the
// engine's resume state: the pre-fx40 census compared only the outermost
// selector's name, so a nested write through the field (the e.resume.outer =
// ... line resolution.go has carried since fx34) never registered. It is
// still a syntactic approximation — an alias bound from e.resume and then
// assigned through would defeat it (see the alias-hole limit recorded in
// TestResumeStateOwnedOnlyByTheResolutionMachinery).
func resumeSelectorInChain(expr ast.Expr) (ast.Expr, bool) {
	for {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}
		if sel.Sel.Name == "resume" {
			return sel, true
		}
		expr = sel.X
	}
}

// funcQualName renders a function declaration the way a review comment would
// name it: the receiver qualified for a method, bare for a free function.
// A generic receiver (never used today) falls back to the bare name.
func funcQualName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	switch t := fd.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return "(" + t.Name + ")." + fd.Name.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return "(*" + id.Name + ")." + fd.Name.Name
		}
	}
	return fd.Name.Name
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "?"
	}
	return cwd
}

// TestEngineCompilesFor32Bit pins portability of the continuation machinery.
// `int` is 64 bits on this box and 32 bits on a 32-bit build, so a
// continuation that packs two counters into one int field — the shape Time
// Travel's resume point first reached for, `(round << 32) | idx` in a
// decision's ResumeTarget — is not merely unportable, it does not compile
// there: the `0xffffffff` mask that unpacks it overflows an untyped int
// constant, and even cast, the shift would discard the high half outright.
// The same class covers any `1 << 31`-and-up constant assigned to an int, a
// len() cast assumed to be 64 bits, and an unsafe.Sizeof assumption.
//
// A cross-compile is the honest test for it, because no amount of running on
// amd64 can observe the narrower word. It costs about a second warm and three
// cold, so it is kept here rather than in the engine packages' own suites.
// GOARCH=386 is the narrowest target the toolchain always ships; CGO is off
// because nothing in the module uses it and a 386 C toolchain is not assumed.
//
// cmd/repro's emit-test probes create scratch packages at the repo root while
// `go test ./...` runs; they are `_`-prefixed (cmd/repro's emitScratch), so
// the `/...` pattern never names them and this build cannot race their
// cleanup.
func TestEngineCompilesFor32Bit(t *testing.T) {
	// `go build` reads the module's source through a subprocess, invisible to
	// go test's result cache. packages() opens every module package directory
	// so a source change invalidates this test's cached result (see its doc
	// comment); the returned map is unused here.
	packages(t)
	env := append(os.Environ(), "GOOS=linux", "GOARCH=386", "CGO_ENABLED=0")
	cmd := exec.Command("go", "build", module+"/...")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("the module does not build for a 32-bit word (GOARCH=386): %v\n%s\n"+
			"an int is 32 bits there; keep two counters in two fields rather than "+
			"packing them into one int, and size any wide constant explicitly", err, out)
	}
}
