package archtest

import (
	"os/exec"
	"regexp"
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
