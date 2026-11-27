package precheckgate

// An OOM-killed `go vet` writes no file:line, and pr-precheck.sh once printed an empty "go vet:" verdict for it. The
// script's vet_reason() is read back out and run on fake logs; the vet scope's cap must also stay at the 8G the dirac
// lockstep tests use.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func vetReason(t *testing.T, rc, log string) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pr-precheck.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?ms)^vet_reason\(\) \{\n(.*?)^\}\n`).FindSubmatch(script)
	if m == nil {
		t.Fatal("scripts/pr-precheck.sh has no vet_reason() function")
	}
	f := filepath.Join(t.TempDir(), "vet.log")
	if err := os.WriteFile(f, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", "-c", "vet_reason() {\n"+string(m[1])+"\n}\nvet_reason \"$1\" \"$2\"", "x", rc, f).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestVetReasonNamesAnOOMKill(t *testing.T) {
	for _, c := range []struct{ rc, log, want string }{
		{"137", "", "killed (memory cap"},
		{"1", "go: error: signal: killed\n", "killed (memory cap"},
		{"1", "# pkg\nengine/rules/a_test.go:12:3: bad printf\n", "engine/rules/a_test.go:12:3"},
		{"2", "\nsomething odd\n\n", "rc 2: something odd"},
	} {
		if got := vetReason(t, c.rc, c.log); !strings.HasPrefix(got, c.want) {
			t.Errorf("vet_reason(%s, %q) = %q, want prefix %q", c.rc, c.log, got, c.want)
		}
	}
}

func TestVetScopeCapIsAtLeast8G(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pr-precheck.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^vetcap=\(.*MemoryMax=(8|9|1\d)G `).Match(script) || !strings.Contains(string(script), `"${vetcap[@]}" go vet`) {
		t.Error("the go vet step must run under vetcap with MemoryMax >= 8G")
	}
}
