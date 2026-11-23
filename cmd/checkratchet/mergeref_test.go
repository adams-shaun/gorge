package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckComparesTheHeadTreeNotTheMergeRef: a pull request's merge ref is the
// branch merged onto current main, while github.event.pull_request.base.sha
// stays at the main tip when the PR was created. If main lowered a row after
// that, the merge ref carries the lowering and the untouched branch must not be
// refused for it. The guard compares the branch's own head tree with the merge
// base instead.
func TestCheckComparesTheHeadTreeNotTheMergeRef(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
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
	row := func(lines int) string {
		return fmt.Sprintf("# key\tlines\tcomplexity\np big\t%d\t70\n", lines)
	}
	write(ceilingsPath, row(400))
	run("init", "-q", "-b", "main")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")

	run("checkout", "-q", "-b", "pr")
	write("README", "the branch changes nothing shape-related\n")
	run("add", "README")
	run("commit", "-q", "-m", "innocent branch change")
	head := run("rev-parse", "HEAD")

	// Main lowers the row after the PR was opened. That lowering is the daily
	// bot's, the only mechanism that lowers rows.
	run("checkout", "-q", "main")
	write(ceilingsPath, row(399))
	run("commit", "-qam", "main lowers after the PR was opened")

	// Check out the merge ref the pull_request event builds, as lint does.
	run("checkout", "-q", "-b", "merge-ref", "main")
	run("merge", "-q", "--no-ff", "pr", "-m", "merge ref")
	got, _, err := check(dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("an untouched branch was refused for main's later lowering: %q", got)
	}

	// The merge ref really does carry the lowering the branch does not, so the
	// fixture fails a guard that reads the checkout.
	ref, err := os.ReadFile(filepath.Join(dir, ceilingsPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ref), "\t399\t") {
		t.Fatalf("merge ref does not carry main's lowered row:\n%s", ref)
	}
	branch, err := git(dir, "show", head+":"+ceilingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(branch, "\t400\t") {
		t.Fatalf("branch head does not carry its base row:\n%s", branch)
	}

	// A branch that really lowers against main's row is still refused.
	run("checkout", "-q", "pr")
	write(ceilingsPath, row(398))
	run("commit", "-qam", "branch lowers below main")
	head = run("rev-parse", "HEAD")
	got, _, err = check(dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "p big") {
		t.Fatalf("lowering against main's row: got %q, want the p big row", got)
	}
}
