package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeFixture writes the smallest tree codeshape.Measure accepts, plus a
// strings scope: engine/rules holds n methods on *Engine (the
// engineMethodCount metric), engine/effects holds the Host, Ctx and
// resumePoint shapes Measure refuses to miss, and bots/azmcts holds n
// strings.* calls (its "strings.* calls" count). The tree is the same at
// both shapes; the caller rewrites it for growth.
func writeFixture(t *testing.T, dir string, methods, stringCalls int) {
	t.Helper()
	var b strings.Builder
	b.WriteString("package rules\n\ntype resumePoint struct{ A, B int }\n\ntype Engine struct{}\n\n")
	for i := 0; i < methods; i++ {
		b.WriteString("func (e *Engine) m" + strconv.Itoa(i) + "() {}\n\n")
	}
	p := filepath.Join(dir, "engine", "rules", "f.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "package effects\n\ntype Host interface{ A() }\n\ntype Ctx struct{ X int }\n"
	p = filepath.Join(dir, "engine", "effects", "f.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// codeshape.Measure refuses a zero trigmatch.Board count (the measurer's
	// sanity guard), so the fixture carries one.
	src = "package trigmatch\n\ntype Board interface{ OnZone() bool }\n"
	p = filepath.Join(dir, "engine", "rules", "trigmatch", "f.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var s strings.Builder
	s.WriteString("package azmcts\n\nimport \"strings\"\n\nfunc find(hay []string) int {\n\tn := 0\n")
	for i := 0; i < stringCalls; i++ {
		s.WriteString("\tif strings.Contains(hay[0], " + strconv.Quote(strconv.Itoa(i)) + ") {\n\t\tn++\n\t}\n")
	}
	s.WriteString("\treturn n\n}\n")
	p = filepath.Join(dir, "bots", "azmcts", "f.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// TestCountGrowthEndToEnd drives the tool over a fixture git repository: a
// base commit holding five *Engine methods and five strings.* calls (each
// allowance is max(2% of 5, 3) = 3, limit 8), a working tree where both grow
// past their allowance (one per metric group: the shape census and the
// string counts), a working tree where both grow within it, and a new scope
// absent at the base. The unchanged tree passes.
func TestCountGrowthEndToEnd(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "countgrowth")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	repo := t.TempDir()
	gitCmd(t, repo, "init", "-q")
	gitCmd(t, repo, "config", "user.email", "t@example.com")
	gitCmd(t, repo, "config", "user.name", "t")
	writeFixture(t, repo, 5, 5)
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

	// The unchanged tree: every metric at its base value, an ok.
	out, code := run()
	if code != 0 || !strings.Contains(out, "countgrowth: ok") {
		t.Fatalf("unchanged tree: exit %d, output %q", code, out)
	}

	// Both groups grow past the allowance (9 > 5 + 3): the shape census names
	// engineMethodCount with its advice, the string count names the scope and
	// metric.
	writeFixture(t, repo, 9, 9)
	out, code = run()
	if code != 1 {
		t.Fatalf("grown tree: exit %d, output %q", code, out)
	}
	for _, want := range []string{
		"engineMethodCount: measured 9 at head, 5 at base, over its allowed limit of 8",
		"Every *Engine method can call every other",
		"bots/azmcts: strings.* calls: measured 9 at head, 5 at base, over its allowed limit of 8",
		"convert the new sites to IDs at the boundary",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}

	// Growth within the allowance (8 = 5 + 3) passes and is reported.
	writeFixture(t, repo, 8, 8)
	out, code = run()
	if code != 0 || !strings.Contains(out, "countgrowth: ok") {
		t.Fatalf("within-allowance tree: exit %d, output %q", code, out)
	}
	if !strings.Contains(out, "ok engineMethodCount 8 (base 5, allowed 8)") {
		t.Errorf("output %q does not report engineMethodCount as ok", out)
	}

	// A scope absent at the base is held to the floor alone: base 0, limit 3,
	// so 4 strings.* calls in a new bots/policynet fail. (An untracked file is
	// counted: the head side is the whole working tree.)
	p := filepath.Join(repo, "bots", "policynet", "f.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	newScope := "package policynet\n\nimport \"strings\"\n\nfunc find(hay []string) bool {\n\treturn strings.Contains(hay[0], \"a\") || strings.Contains(hay[1], \"b\") || strings.Contains(hay[2], \"c\") || strings.Contains(hay[3], \"d\")\n}\n"
	if err := os.WriteFile(p, []byte(newScope), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = run()
	if code != 1 {
		t.Fatalf("new-scope tree: exit %d, output %q", code, out)
	}
	if !strings.Contains(out, "bots/policynet: strings.* calls: measured 4 at head with no value at the merge base") {
		t.Errorf("output %q does not report the new scope", out)
	}

	// The growth taken back: ok again. Shrinking never fails.
	if err := os.RemoveAll(filepath.Join(repo, "bots", "policynet")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, repo, 3, 3)
	if out, code = run(); code != 0 || !strings.Contains(out, "countgrowth: ok") {
		t.Fatalf("shrunk tree: exit %d, output %q", code, out)
	}
}
