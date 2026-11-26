package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bigFunc renders one function spanning n source lines (filler comments, no
// branches: complexity 1).
func bigFunc(name string, n int) string {
	var b strings.Builder
	b.WriteString("func " + name + "() int {\n")
	for i := 3; i < n; i++ {
		b.WriteString("\t// filler\n")
	}
	b.WriteString("\treturn 1\n}\n")
	return b.String()
}

// writeTree writes engine/rules/f.go holding bigA and bigB at the given
// lengths (the tool measures only codeshape.ShapeDirs).
func writeTree(t *testing.T, dir string, a, b int) {
	t.Helper()
	src := "package p\n\n" + bigFunc("bigA", a) + "\n" + bigFunc("bigB", b)
	p := filepath.Join(dir, "engine", "rules", "f.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitCmd runs a git command in the fixture repo.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// TestFuncGrowthEndToEnd drives the tool over a fixture git repository: a
// base commit holding two functions just over codeshape.LongFuncLines, a
// working tree where one grows past its allowance (5% of 310 = 15, limit 325)
// and one within it, then the same tree with the growth taken back.
func TestFuncGrowthEndToEnd(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "funcgrowth")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	repo := t.TempDir()
	gitCmd(t, repo, "init", "-q")
	gitCmd(t, repo, "config", "user.email", "t@example.com")
	gitCmd(t, repo, "config", "user.name", "t")
	// 310 lines at base: over the 300-line threshold, so both functions are
	// held; their allowance is max(5%, 10) = 15 lines, limit 325.
	writeTree(t, repo, 310, 310)
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "base")

	run := func() (string, int) {
		cmd := exec.Command(bin, "-base", "HEAD")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run: %v: %s", err, out)
		}
		return string(out), code
	}

	// No changed file between the base and the worktree yet: an ok.
	out, code := run()
	if code != 0 || !strings.Contains(out, "0 changed files against base") || !strings.Contains(out, "funcgrowth: ok") {
		t.Fatalf("unchanged tree: exit %d, output %q", code, out)
	}

	// bigA to 330 (past 325) fails and names the function, its file, the base,
	// the head and the limit; bigB to 320 is within its allowance.
	writeTree(t, repo, 330, 320)
	out, code = run()
	if code != 1 {
		t.Fatalf("grown tree: exit %d, output %q", code, out)
	}
	for _, want := range []string{"bigA", "f.go", "grew to lines 330 from 310 at base", "allowed limit of 325"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
	if !strings.Contains(out, "ok bigB") || !strings.Contains(out, "320 (base 310, allowed 325)") {
		t.Errorf("output %q does not report bigB as ok", out)
	}

	// The growth taken back: ok again, and shrinking never fails. Both
	// functions are under the threshold at head now, so neither is checked.
	writeTree(t, repo, 300, 290)
	out, code = run()
	if code != 0 || !strings.Contains(out, "funcgrowth: ok") || strings.Contains(out, "ok bigA") {
		t.Fatalf("shrunk tree: exit %d, output %q", code, out)
	}
}
