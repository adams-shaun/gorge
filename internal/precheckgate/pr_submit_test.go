package precheckgate

// scripts/pr-submit.sh is the one way a hand or subagent branch opens its PR to main (11-26: 9 of 10 red PRs were
// subagent wt/* branches that skipped pr-derived, pr-precheck and the merge of main). The tests run the real script
// in a temporary origin, main checkout and worktree, with stub pr-derived.sh (a rise: main's support.md), stub
// pr-precheck.sh (ok unless PRECHECK_FAIL is set) and a stub gh on PATH; pr-autoresolve.sh and its path list are the
// real ones, since the script asks them which conflicts are generated.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type submitRepo struct {
	root, origin, main, wt, ghLog string
}

func (r submitRepo) write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// newSubmitRepo builds origin (bare), a main checkout on main, and a worktree on branch feat cut from it.
func newSubmitRepo(t *testing.T) submitRepo {
	t.Helper()
	root := t.TempDir()
	r := submitRepo{root: root, origin: filepath.Join(root, "origin.git"), main: filepath.Join(root, "main"),
		wt: filepath.Join(root, "wt"), ghLog: filepath.Join(root, "gh.log")}
	for _, f := range []string{"pr-submit.sh", "pr-autoresolve.sh", "autoresolve-paths.tsv"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "scripts", f))
		if err != nil {
			t.Fatal(err)
		}
		r.write(t, filepath.Join(r.main, "scripts", f), string(b))
	}
	r.write(t, filepath.Join(r.main, "scripts", "pr-derived.sh"),
		"#!/usr/bin/env bash\ngit checkout -q \"$1\" -- dirac/docs/support.md\nexit 0\n")
	r.write(t, filepath.Join(r.main, "scripts", "pr-precheck.sh"),
		"#!/usr/bin/env bash\nif [ -n \"${PRECHECK_FAIL:-}\" ]; then echo 'boom in codeshape'; echo 'pr-precheck: codeshape: boom'; exit 1; fi\necho 'pr-precheck: ok'\n")
	r.write(t, filepath.Join(r.main, "scripts", "pr-regen.sh"),
		"#!/usr/bin/env bash\necho \"regen $* head=$PR_REGEN_HEAD\" >\"$GH_LOG.regen\"\n")
	r.write(t, filepath.Join(r.main, "a.txt"), "base\n")
	r.write(t, filepath.Join(r.main, "dirac", "docs", "support.md"), "supported 10\n")
	r.write(t, filepath.Join(root, "bin", "gh"), `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$GH_LOG"
case "$1 $2" in
"pr list") [ -f "$GH_OPEN" ] && cat "$GH_OPEN"; exit 0 ;;
"pr create") echo https://github.com/o/r/pull/77 ;;
esac
`)
	r.write(t, filepath.Join(root, "empty.gitconfig"), "")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", r.origin)
	gitIn(t, root, "init", "-q", "-b", "main", r.main)
	gitIn(t, r.main, "add", ".")
	gitIn(t, r.main, "commit", "-q", "-m", "base")
	gitIn(t, r.main, "remote", "add", "origin", r.origin)
	gitIn(t, r.main, "push", "-q", "origin", "main")
	gitIn(t, r.main, "worktree", "add", "-q", "-b", "feat", r.wt)
	return r
}

// commit writes files in dir and commits them.
func (r submitRepo) commit(t *testing.T, dir, msg string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		r.write(t, filepath.Join(dir, p), body)
		gitIn(t, dir, "add", p)
	}
	gitIn(t, dir, "commit", "-q", "-m", msg)
}

// moveMain lands a commit on origin/main from a scratch clone, leaving the main checkout alone.
func (r submitRepo) moveMain(t *testing.T, files map[string]string) {
	t.Helper()
	c := filepath.Join(r.root, "mover")
	if _, err := os.Stat(c); err != nil {
		gitIn(t, r.root, "clone", "-q", r.origin, c)
	} else {
		gitIn(t, c, "pull", "-q")
	}
	r.commit(t, c, "main moves", files)
	gitIn(t, c, "push", "-q", "origin", "main")
}

func (r submitRepo) run(t *testing.T, dir string, env ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", "scripts/pr-submit.sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(r.root, "bin")+":"+os.Getenv("PATH"),
		"GIT_CONFIG_GLOBAL="+filepath.Join(r.root, "empty.gitconfig"), "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GH_LOG="+r.ghLog, "GH_OPEN="+filepath.Join(r.root, "gh-open"))
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	}
	t.Fatal(err)
	return "", -1
}

func (r submitRepo) remoteHasBranch(t *testing.T) bool {
	t.Helper()
	return exec.Command("git", "-C", r.origin, "rev-parse", "-q", "--verify", "refs/heads/feat").Run() == nil
}

func TestPrSubmitResetsDerivedChangeAndOpensOnce(t *testing.T) {
	r := newSubmitRepo(t)
	mainHead := gitOut(t, r.main, "rev-parse", "HEAD")
	r.commit(t, r.wt, "feat: add b", map[string]string{"b.txt": "b\n", "dirac/docs/support.md": "supported 11\n"})
	r.moveMain(t, map[string]string{"c.txt": "c\n"})
	out, rc := r.run(t, r.wt)
	if rc != 0 || !strings.HasSuffix(strings.TrimSpace(out), "pr-submit: ok #77") {
		t.Fatalf("rc=%d, want ok #77:\n%s", rc, out)
	}
	if got := gitOut(t, r.origin, "show", "feat:dirac/docs/support.md"); got != "supported 10" {
		t.Errorf("pushed support.md = %q, want main's copy (pr-derived's reset)", got)
	}
	for _, f := range []string{"b.txt", "c.txt"} {
		if gitOut(t, r.origin, "ls-tree", "--name-only", "feat", f) != f {
			t.Errorf("pushed feat lacks %s (the branch's change, or main merged in)", f)
		}
	}
	log, _ := os.ReadFile(r.ghLog)
	if !strings.Contains(string(log), "pr create --base main --head feat") || !strings.Contains(string(log), "Shan't Be Blamed") ||
		strings.Contains(string(log), "--draft") {
		t.Errorf("gh calls lack a non-draft create to main with the signature:\n%s", log)
	}
	// Idempotent: with the PR open, a rerun re-syncs and pushes, and creates nothing.
	r.write(t, filepath.Join(r.root, "gh-open"), "77\n")
	r.moveMain(t, map[string]string{"d.txt": "d\n"})
	out, rc = r.run(t, r.wt)
	if rc != 0 || !strings.Contains(out, "to the open #77") {
		t.Fatalf("rerun rc=%d, want a push to the open #77:\n%s", rc, out)
	}
	if log, _ := os.ReadFile(r.ghLog); strings.Count(string(log), "pr create") != 1 {
		t.Errorf("the rerun created a second PR:\n%s", log)
	}
	if gitOut(t, r.origin, "ls-tree", "--name-only", "feat", "d.txt") != "d.txt" {
		t.Error("the rerun did not merge main's new commit into the pushed branch")
	}
	if gitOut(t, r.main, "rev-parse", "HEAD") != mainHead || gitOut(t, r.main, "status", "--porcelain") != "" {
		t.Error("the main checkout moved or got dirty")
	}
}

func TestPrSubmitRefusesFailedPrecheck(t *testing.T) {
	r := newSubmitRepo(t)
	r.commit(t, r.wt, "feat: add b", map[string]string{"b.txt": "b\n"})
	out, rc := r.run(t, r.wt, "PRECHECK_FAIL=1")
	if rc != 1 || !strings.Contains(out, "boom in codeshape") || !strings.Contains(out, "pr-precheck failed") {
		t.Fatalf("rc=%d, want 1 with precheck's own output:\n%s", rc, out)
	}
	if r.remoteHasBranch(t) {
		t.Error("a branch whose precheck failed was pushed")
	}
}

func TestPrSubmitRefusesConflictOutsideDerivedFiles(t *testing.T) {
	r := newSubmitRepo(t)
	r.commit(t, r.wt, "feat: edit a", map[string]string{"a.txt": "branch\n"})
	r.moveMain(t, map[string]string{"a.txt": "main\n"})
	out, rc := r.run(t, r.wt)
	if rc != 4 || !strings.Contains(out, "  a.txt") {
		t.Fatalf("rc=%d, want 4 naming a.txt:\n%s", rc, out)
	}
	if _, err := os.Stat(filepath.Join(gitOut(t, r.wt, "rev-parse", "--absolute-git-dir"), "MERGE_HEAD")); err == nil {
		t.Error("the conflicting merge was left in progress")
	}
	if r.remoteHasBranch(t) {
		t.Error("a conflicting branch was pushed")
	}
}

func TestPrSubmitRefusesTheMainCheckout(t *testing.T) {
	r := newSubmitRepo(t)
	gitIn(t, r.main, "checkout", "-q", "-b", "other")
	r.commit(t, r.main, "other", map[string]string{"b.txt": "b\n"})
	head := gitOut(t, r.main, "rev-parse", "HEAD")
	out, rc := r.run(t, r.main)
	if rc != 2 || !strings.Contains(out, "is the main checkout") {
		t.Fatalf("rc=%d, want 2 refusing the main checkout:\n%s", rc, out)
	}
	if gitOut(t, r.main, "rev-parse", "HEAD") != head || gitOut(t, r.main, "status", "--porcelain") != "" {
		t.Error("the main checkout moved or got dirty")
	}
}

func TestPrSubmitRebuildsAGeneratedConflict(t *testing.T) {
	r := newSubmitRepo(t)
	r.commit(t, r.wt, "feat: add b", map[string]string{"b.txt": "b\n", "dirac/docs/support.md": "supported 11\n"})
	pre := gitOut(t, r.wt, "rev-parse", "HEAD")
	r.moveMain(t, map[string]string{"dirac/docs/support.md": "supported 12\n"})
	out, rc := r.run(t, r.wt)
	if rc != 0 {
		t.Fatalf("rc=%d, want a generated-only conflict resolved:\n%s", rc, out)
	}
	regen, _ := os.ReadFile(r.ghLog + ".regen")
	if want := "regen dirac/docs/support.md head=" + pre; strings.TrimSpace(string(regen)) != want {
		t.Errorf("pr-regen.sh call %q, want %q", strings.TrimSpace(string(regen)), want)
	}
	if got := gitOut(t, r.origin, "show", "feat:dirac/docs/support.md"); got != "supported 12" {
		t.Errorf("pushed support.md = %q, want main's copy rebuilt", got)
	}
}
