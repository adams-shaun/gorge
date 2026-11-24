package precheckgate

// pr-precheck.sh gates its engine/rules golden-head and corpus-ratchet step on a `heads_gate` function it defines
// itself; the script is the single home of the pattern, so these tests read the function back out of the script and
// run it on fake `git diff --name-only` lists (the same read-back shape scripts/dirac_gate_test.py uses for
// dirac-gate.sh). The gate exists so a change that can move the goldens — engine/{events,effects,state,rules,cards}
// or cmd/forgec — selects the ratchet step even though no affected-package rule reaches ./engine/rules (#74, #75:
// two PRs said ok and were dequeued by their batches on TestSetHeads and TestTakeInitiative).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	selectLine = "select"
	skipLine   = "skip"
)

// headsGate extracts the script's heads_gate() function and returns a runner that feeds it a fake diff list on
// stdin and reports whether the ratchet step is selected.
func headsGate(t *testing.T) func(diff string) bool {
	t.Helper()
	return scriptGate(t, "heads_gate")
}

// scriptGate extracts the named gate function from pr-precheck.sh and returns a runner that feeds it a fake diff
// list on stdin and reports whether its step is selected.
func scriptGate(t *testing.T, name string) func(diff string) bool {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pr-precheck.sh"))
	if err != nil {
		t.Fatalf("read scripts/pr-precheck.sh: %v", err)
	}
	m := regexp.MustCompile(`(?ms)^` + name + `\(\) \{\n(.*?)^\}\n`).FindSubmatch(script)
	if m == nil {
		t.Fatalf("scripts/pr-precheck.sh has no %s() function (or it does not end in a bare })", name)
	}
	sh := "set -o pipefail\n" + name + "() {\n" + string(m[1]) + "\n}\nif " + name + "; then echo " + selectLine + "; else echo " + skipLine + "; fi\n"
	return func(diff string) bool {
		cmd := exec.Command("bash", "-c", sh)
		cmd.Stdin = strings.NewReader(diff)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("run %s on %q: %v", name, diff, err)
		}
		switch line := strings.TrimSpace(string(out)); line {
		case selectLine:
			return true
		case skipLine:
			return false
		default:
			t.Fatalf("%s printed %q, want exactly %s or %s", name, line, selectLine, skipLine)
			return false
		}
	}
}

func TestHeadsGateSelectsEngineAndForgecPaths(t *testing.T) {
	gate := headsGate(t)
	for _, path := range []string{
		"engine/events/crew.go",
		"engine/effects/foo_params.go",
		"engine/state/state.go",
		"engine/rules/cast_begin.go",
		"engine/cards/set.txt",
		"cmd/forgec/main.go",
		// The golden files themselves sit under testdata: a hand-edited golden must select the step too.
		"engine/rules/testdata/setheads/heads/fin-2.txt",
	} {
		if !gate(path + "\n") {
			t.Errorf("heads_gate skipped %q, want select", path)
		}
	}
}

func TestHeadsGateSkipsUnrelatedPaths(t *testing.T) {
	gate := headsGate(t)
	for _, path := range []string{
		"docs/x.md",
		"docs/agents/known-approximations/register.md",
		"scripts/pr-precheck.sh",
		"internal/codeshape/shape.go",
		"web/src/App.svelte",
	} {
		if gate(path + "\n") {
			t.Errorf("heads_gate selected %q, want skip", path)
		}
	}
}

func TestHeadsGateSelectsWhenMixedWithUnrelatedPaths(t *testing.T) {
	gate := headsGate(t)
	diff := "docs/x.md\nscripts/ci-corpus.sh\nengine/events/crew.go\n"
	if !gate(diff) {
		t.Error("heads_gate skipped a diff mixing docs and engine/events, want select")
	}
	if gate("engine/events.go\n") {
		t.Error("heads_gate selected engine/events.go (a file outside engine/events/), want skip")
	}
}
