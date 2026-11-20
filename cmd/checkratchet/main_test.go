package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckRefusesALoweredConstantAndRow runs the guard's enumeration over a real repository:
// a branch that lowers a constant in a nested ratchet_test.go, or a ceilings row, is reported; a
// branch that leaves both alone is not. ls-tree takes no glob, so a pathspec for ratchet_test.go
// matched nothing and the constant half never ran.
func TestCheckRefusesALoweredConstantAndRow(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const constFile = "x/y/ratchet_test.go"
	write(constFile, "package y\n\nconst limit = 400\n")
	write(ceilingsPath, "# key\tlines\tcomplexity\np big\t400\t70\n")
	run("init", "-q", "-b", "main")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "branch")

	if lowered, _, err := check(dir, "main", "HEAD"); err != nil || len(lowered) != 0 {
		t.Fatalf("an unchanged branch: lowered %q, err %v; want none", lowered, err)
	}

	write(constFile, "package y\n\nconst limit = 399\n")
	run("commit", "-qam", "lower the constant")
	lowered, _, err := check(dir, "main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(lowered) != 1 || !strings.HasPrefix(lowered[0], constFile+": ") {
		t.Fatalf("lowered constant: got %q, want one entry for %s", lowered, constFile)
	}

	write(constFile, "package y\n\nconst limit = 400\n")
	write(ceilingsPath, "# key\tlines\tcomplexity\np big\t399\t70\n")
	run("commit", "-qam", "lower the row")
	lowered, _, err = check(dir, "main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(lowered) != 1 || !strings.HasPrefix(lowered[0], ceilingsPath+": ") {
		t.Fatalf("lowered row: got %q, want one entry for %s", lowered, ceilingsPath)
	}

	run("rm", "-q", constFile)
	write(ceilingsPath, "# key\tlines\tcomplexity\np big\t400\t70\n")
	run("commit", "-qam", "remove the ratchet file")
	lowered, _, err = check(dir, "main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(lowered) != 1 || lowered[0] != constFile+": file removed" {
		t.Fatalf("removed ratchet file: got %q", lowered)
	}
}
