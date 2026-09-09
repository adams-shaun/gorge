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
	"testing"
)

const module = "github.com/adams-shaun/gorge"

type pkg struct {
	path    string
	imports map[string]bool // direct, non-test
	deps    map[string]bool // transitive, non-test
}

// packages lists every package in the module with its direct and transitive
// non-test imports. Test files are excluded on purpose: tests may import
// anything (view's tests import rules; cards' tests import time).
func packages(t *testing.T) map[string]pkg {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}|{{join .Deps " "}}`, module+"/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	pkgs := map[string]pkg{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			t.Fatalf("unexpected go list line %q", line)
		}
		p := pkg{path: parts[0], imports: set(parts[1]), deps: set(parts[2])}
		pkgs[p.path] = p
	}
	if len(pkgs) < 10 {
		t.Fatalf("go list found only %d packages", len(pkgs))
	}
	return pkgs
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
// cmd/testtime is exempt because it is a build-time developer tool, not part
// of the engine or the server: it stamps each TEST_HISTORY.md row with the
// UTC time the measurement was taken. Nothing it produces reaches a game, an
// event, a view or a replay, so D16's determinism argument does not apply.
func TestTimeIsImportedOnlyByTheHost(t *testing.T) {
	allowed := map[string]bool{
		module + "/host":         true,
		module + "/host/httpapi": true,
		module + "/cmd/gorged":   true,
		module + "/cmd/testtime": true,
	}
	for path, p := range packages(t) {
		if p.imports["time"] && !allowed[path] {
			t.Errorf("%s imports time; only host, host/httpapi and cmd/gorged may", path)
		}
	}
}

// TestDependencyOrderHolds pins the arrows that must never appear, direct or
// transitive.
func TestDependencyOrderHolds(t *testing.T) {
	pkgs := packages(t)
	forbidden := []struct{ from, to string }{
		{module + "/effects", module + "/rules"},
		{module + "/view", module + "/rules"},
		{module + "/botpolicy", module + "/view"},
		{module + "/botpolicy", module + "/rules"},
		{module + "/botpolicy", module + "/seat"},
		{module + "/protocol", module + "/rules"},
		{module + "/host", module + "/internal/testutil"},
		{module + "/host/httpapi", module + "/internal/testutil"},
		{module + "/cmd/gorged", module + "/internal/testutil"},
		{module + "/cards", module + "/state"},
		{module + "/deck", module + "/rules"},
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
}

// TestNoExportLeaksAnEngineGame is D6's compile-time half: no exported
// function or method of host or host/httpapi may expose a *state.Game or a
// *rules.Engine through its signature — by return type, or by any parameter
// shape a caller could funnel back into a leaked live game. Every value that
// crosses either package is already a view.View, a protocol.* or a
// decision.* payload; the engine and its game never leave rules/ at all (a
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
	// These exact type names (state.Game, rules.Engine) are what a leak's
	// signature would carry; a legitimate client-facing type never has an
	// element in either package. Word boundaries stop the match from
	// prefix-colliding with a hypothetical longer identifier.
	leaks := []*regexp.Regexp{
		regexp.MustCompile(`\bstate\.Game\b`),
		regexp.MustCompile(`\brules\.Engine\b`),
	}
	for _, pkg := range []string{module + "/host", module + "/host/httpapi"} {
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
// only randomness (rules/rng.go, seat/bot.go). The v1 package's global
// functions are exactly the ambient randomness the engine spec forbids.
func TestNoLegacyMathRand(t *testing.T) {
	for path, p := range packages(t) {
		if p.imports["math/rand"] {
			t.Errorf("%s imports math/rand; use math/rand/v2 with a seeded source", path)
		}
	}
}

// resumeFieldWriters walks the non-test rules/ sources and returns every
// function that assigns to the Engine.resume field, keyed by the receiver-
// qualified function name (e.g. "(*Engine).Ask") with the sites it writes.
// It is the census that TestResumeStateOwnedOnlyByTheResolutionMachinery
// both enforces against and keeps exact.
func resumeFieldWriters(t *testing.T) map[string][]string {
	t.Helper()
	dir := filepath.Join("..", "..", "rules")
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no rules sources found under %s (cwd %s)", dir, mustCwd())
	}
	out := map[string][]string{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
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
		rel := filepath.Join("rules", filepath.Base(file))
		ast.Inspect(f, func(n ast.Node) bool {
			check := func(target ast.Expr) {
				sel, ok := target.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "resume" {
					return
				}
				fn := enclosing(sel.Pos())
				out[fn] = append(out[fn], fmt.Sprintf("%s:%d", rel, fset.Position(sel.Pos()).Line))
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

// TestResumeStateOwnedOnlyByTheResolutionMachinery is ruling T21-e's static,
// enforced half for the mid-resolution ask. An asking primitive suspends by
// calling effects.Host.Ask and returning true; only the resolution machinery
// (rules/resolution.go: Ask, handleModes, resumeResolution) and the CR 800.4f
// departed-player release hook (rules/sba.go: releasePendingDecisionOfDepartedPlayer)
// may set or clear e.resume. A write from anywhere else bypasses the event
// emit and is invisible to a log-only replay — exactly the class of bug that
// used to be caught only by a human reading a diff.
//
// The allow-list is the exact census measured at the time the rule was
// written (fx38). Both directions are pinned: a writer not on the list fails,
// and a listed function that no longer writes also fails, so adding a new
// writer is a deliberate, reviewed edit to the allow-list rather than a
// silent pass, and pruning a dead entry is equally deliberate.
func TestResumeStateOwnedOnlyByTheResolutionMachinery(t *testing.T) {
	allowed := map[string]string{
		"(*Engine).Ask":         "the asking primitive: records the resume point for the answer machine to re-enter (rules/resolution.go)",
		"(*Engine).handleModes": "the KModes answer handler: clears the resume point then re-enters via resumeResolution (rules/resolution.go)",
		"(*Engine).Clone":       "a snapshot clone copies the resume point onto the freshly-cloned engine, not the live one (rules/clone.go)",
		"(*Engine).releasePendingDecisionOfDepartedPlayer": "CR 800.4f: a departed player's outstanding ask is released with an empty answer (rules/sba.go)",
	}
	writers := resumeFieldWriters(t)

	// Enforcement: a direct write from a function that does not own the
	// engine's resume state is a replay-visibility bug. Fail naming the
	// offending symbol and the ruling.
	for fn, sites := range writers {
		if _, ok := allowed[fn]; ok {
			continue
		}
		for _, site := range sites {
			t.Errorf("%s: %s writes the engine resume state directly; "+
				"only the resolution machinery or the CR 800.4f departed-player hook may "+
				"set or clear it (ruling T21-e: state changes route through an event emit)",
				site, fn)
		}
	}

	// Census exactness: keep the allow-list in lockstep with reality so a
	// write that moves between functions is a deliberate review, and a dead
	// entry is pruned rather than silently trusted.
	for fn := range allowed {
		if _, ok := writers[fn]; !ok {
			t.Errorf("allow-list entry %s is stale: it writes no resume field; prune it or the write moved (ruling T21-e)", fn)
		}
	}

	// The census is worth recording even when every writer is legitimate:
	// the rule only means something if the list of observed writers is the
	// finite set we audited. Sort for a deterministic failure message.
	named := make([]string, 0, len(writers))
	for fn := range writers {
		named = append(named, fn)
	}
	sort.Strings(named)
	for _, fn := range named {
		t.Logf("resume writer %s at %s", fn, strings.Join(writers[fn], ", "))
	}
}
