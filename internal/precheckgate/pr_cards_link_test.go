package precheckgate

// scripts/pr-cards-link.sh gives a pr-<id> worktree its corpus. A branch that bumps FORGE_REF must not inherit the
// main checkout's corpus at the old pin (testutil.SharedCorpusDir panics on it, 2026-11-24): it takes the per-ref
// store scripts/link-cards.sh builds. The test runs the real script in a temporary main checkout plus worktree,
// with a stub link-cards.sh that records its call.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// linkWorktree builds a main checkout whose corpus lock names mainRef and a worktree whose Makefile pins wtRef, runs
// pr-cards-link.sh there, and reports whether the stub link-cards.sh ran and where .cards points.
func linkWorktree(t *testing.T, mainRef, wtRef string) (stubRan bool, target string) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "pr-cards-link.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	main := filepath.Join(root, "main")
	wt := filepath.Join(root, "wt")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, root, "init", "-q", main)
	write(filepath.Join(main, "Makefile"), "FORGE_REF  ?= "+wtRef+"\n")
	write(filepath.Join(main, "scripts", "link-cards.sh"), "#!/usr/bin/env bash\ntouch \"$PWD/stub-ran\"\nmkdir -p store/cardsfolder && ln -sfn \"$PWD/store\" .cards\n")
	write(filepath.Join(main, ".cards", "cardsfolder", "x.txt"), "x\n")
	write(filepath.Join(main, ".cards", "cards.lock"), `{"ref": "`+mainRef+`"}`+"\n")
	gitIn(t, main, "add", "Makefile", "scripts")
	gitIn(t, main, "commit", "-q", "-m", "base")
	gitIn(t, main, "worktree", "add", "-q", wt)
	cmd := exec.Command("bash", script)
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pr-cards-link.sh: %v\n%s", err, out)
	}
	_, statErr := os.Lstat(filepath.Join(wt, "stub-ran"))
	target, err = os.Readlink(filepath.Join(wt, ".cards"))
	if err != nil {
		t.Fatalf(".cards is not a link: %v", err)
	}
	return statErr == nil, strings.TrimSuffix(target, "/")
}

func TestPrCardsLinkBumpedPinGetsItsOwnCorpus(t *testing.T) {
	ran, target := linkWorktree(t, "aaaa111", "bbbb222")
	if !ran {
		t.Errorf("the worktree pins bbbb222 but the main corpus holds aaaa111: link-cards.sh must run, .cards -> %s", target)
	}
	if strings.HasSuffix(target, filepath.Join("main", ".cards")) {
		t.Errorf(".cards -> %s: the main checkout's corpus at another pin", target)
	}
}

func TestPrCardsLinkSamePinSharesTheMainCorpus(t *testing.T) {
	ran, target := linkWorktree(t, "cccc333", "cccc333")
	if ran {
		t.Error("the main corpus carries the worktree's pin: link-cards.sh must not run")
	}
	if !strings.HasSuffix(target, filepath.Join("main", ".cards")) {
		t.Errorf(".cards -> %s, want the main checkout's corpus", target)
	}
}
