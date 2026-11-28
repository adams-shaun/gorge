package scripts

// TestOracleRefuseInstead drives scripts/oracle-refuse-instead.sh on a throwaway repository with a culprit merge,
// a stub refuse-on-red.sh and a stub verify command: which outcome each refuse-on-red answer and verify result
// turns into, and that every non-zero exit leaves the tree at the starting head with the branch gone.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const refuseInsteadBranch = "hotfix/auto-refuse-4559"

// refuseInsteadRorStub stands in for refuse-on-red.sh. It logs its arguments and REFUSE_ON_RED, then acts as
// STUB_ROR says: na, person, nothing (exit 0, no commit) or refuse (commits one refusal, then leaves a modified
// tracked file and an untracked file under dirac/support behind, as a failed later step might).
const refuseInsteadRorStub = `#!/usr/bin/env bash
echo "args=$* mode=${REFUSE_ON_RED:-}" >> "$STUB_LOG"
case $STUB_ROR in
na) echo "refuse-on-red: n/a (no Rule: CR N or Oracle: ... line in a commit message)" ;;
person) echo "refuse-on-red: needs a person: 13 cards is more than REFUSE_ON_RED_MAX (12)"; exit 1 ;;
nothing) echo "refuse-on-red: lockstep holds (300 pairs a slice)" ;;
refuse)
	mkdir -p dirac/support/refused
	echo "refused: Card A" > dirac/support/refused/Card-A.txt
	git add -A -- dirac
	git commit -q -m "dirac: refuse 1 cards the rules fix diverges on" -m "Rule: CR 601.2" -- dirac
	echo "go run ./dirac/kgen -refuse \"Card A\" -why \"gorge follows Rule: CR 601.2; dirac not yet\""
	echo "refuse-on-red: refused 1 cards"
	echo "stray" > dirac/support/stray.txt
	echo "changed" > dirac/support/tracked.txt
	;;
esac
`

type refuseInsteadRig struct {
	repo, culprit, head, log string
}

func newRefuseInsteadRig(t *testing.T) *refuseInsteadRig {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	r := &refuseInsteadRig{repo: filepath.Join(root, "repo"), log: filepath.Join(root, "stub.log")}
	g := func(args ...string) string {
		return strings.TrimSpace(sh(t, r.repo, "git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...))
	}
	files := map[string]string{
		"scripts/oracle-refuse-instead.sh":           readText(t, "oracle-refuse-instead.sh"),
		"scripts/sentinel-append.sh":                 readText(t, "sentinel-append.sh"),
		"scripts/refuse-on-red.sh":                   refuseInsteadRorStub,
		"dirac/oracle/testdata/sentinels.txt":        "# sentinels\nTestOld\n",
		"dirac/oracle/testdata/ci-shard-seconds.txt": "# <test> <seconds>\n",
		"dirac/oracle/a_test.go":                     "package oracle\n\nimport \"testing\"\n\nfunc TestOld(t *testing.T) {}\n\nfunc TestAlpha(t *testing.T) {}\n",
		"dirac/support/tracked.txt":                  "kept\n",
	}
	for p, c := range files {
		write(t, filepath.Join(r.repo, p), c, 0o755)
	}
	sh(t, root, "git", "init", "-q", "-b", "main", r.repo)
	g("add", "-A")
	g("commit", "-q", "-m", "base")
	g("checkout", "-q", "-b", "pr4559")
	write(t, filepath.Join(r.repo, "engine/rules/fix.go"), "package rules\n", 0o644)
	g("add", "-A")
	g("commit", "-q", "-m", "fix(rules): keep the card\n\nRule: CR 601.2")
	g("checkout", "-q", "main")
	g("merge", "-q", "--no-ff", "-m", "Merge pull request #4559 from x/pr4559", "pr4559")
	r.culprit = g("rev-parse", "HEAD")
	write(t, filepath.Join(r.repo, "c.txt"), "later\n", 0o644)
	g("add", "-A")
	g("commit", "-q", "-m", "later work on main")
	r.head = g("rev-parse", "HEAD")
	// Untracked, as in the job: oracle-bisect.sh writes it into the checkout.
	write(t, filepath.Join(r.repo, "failing.txt"), "TestAlpha\nTestBeta\nnot a name\n", 0o644)
	return r
}

// run runs the script as the step does and returns its stdout, stderr and exit code.
func (r *refuseInsteadRig) run(t *testing.T, ror, verify string) (string, string, int) {
	t.Helper()
	c := exec.Command("bash", "scripts/oracle-refuse-instead.sh", r.culprit, "failing.txt", refuseInsteadBranch)
	c.Dir = r.repo
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"STUB_ROR="+ror, "STUB_LOG="+r.log, "PR=4559", "GITHUB_RUN_ID=45740776172",
		"ORACLE_REFUSE_BUILD=echo build >> "+r.log,
		"ORACLE_REFUSE_VERIFY=echo \"verify $RX\" >> "+r.log+"; "+verify)
	var out, errb strings.Builder
	c.Stdout, c.Stderr = &out, &errb
	err := c.Run()
	code := 0
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		code = ee.ExitCode()
	case err != nil:
		t.Fatal(err)
	}
	return out.String(), errb.String(), code
}

func (r *refuseInsteadRig) git(t *testing.T, args ...string) string {
	t.Helper()
	return strings.TrimSpace(sh(t, r.repo, "git", args...))
}

// assertRestored checks the tree is back at the starting head: same commit, no bot branch, and nothing modified,
// staged or left untracked but failing.txt.
func (r *refuseInsteadRig) assertRestored(t *testing.T) {
	t.Helper()
	if got := r.git(t, "rev-parse", "HEAD"); got != r.head {
		t.Errorf("HEAD = %s, want the starting head %s", got, r.head)
	}
	if got := r.git(t, "for-each-ref", "--format=%(refname)", "refs/heads"); got != "refs/heads/main\nrefs/heads/pr4559" {
		t.Errorf("branches after: %q, want main and pr4559 only", got)
	}
	if got := r.git(t, "status", "--porcelain"); got != "?? failing.txt" {
		t.Errorf("status after: %q, want only the untracked failing.txt", got)
	}
}

func TestOracleRefuseInstead(t *testing.T) {
	t.Parallel()

	t.Run("not-rule-cited-exit-10", func(t *testing.T) {
		t.Parallel()
		r := newRefuseInsteadRig(t)
		out, errs, code := r.run(t, "na", "true")
		if code != 10 {
			t.Fatalf("exit %d, want 10\nstdout:\n%s\nstderr:\n%s", code, out, errs)
		}
		if want := "oracle-refuse-instead: n/a (no Rule: CR N or Oracle: ... line in a commit message)\n"; out != want {
			t.Errorf("stdout = %q, want %q", out, want)
		}
		log := readText(t, r.log)
		if !strings.Contains(log, "args=--culprit "+r.culprit+" mode=apply") {
			t.Errorf("refuse-on-red must run in apply mode on the culprit: %q", log)
		}
		if strings.Contains(log, "verify") {
			t.Errorf("n/a must not run the failing tests: %q", log)
		}
		r.assertRestored(t)
	})

	t.Run("no-refusal-exit-11", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ ror, last string }{
			{"person", "refuse-on-red: needs a person: 13 cards is more than REFUSE_ON_RED_MAX (12)"},
			{"nothing", "refuse-on-red: lockstep holds (300 pairs a slice)"},
		} {
			r := newRefuseInsteadRig(t)
			out, errs, code := r.run(t, tc.ror, "true")
			if code != 11 {
				t.Fatalf("%s: exit %d, want 11\nstdout:\n%s\nstderr:\n%s", tc.ror, code, out, errs)
			}
			if want := "oracle-refuse-instead: no refusal (" + tc.last + ")\n"; out != want {
				t.Errorf("%s: stdout = %q, want %q", tc.ror, out, want)
			}
			r.assertRestored(t)
		}
	})

	t.Run("refusal-does-not-cure-exit-12", func(t *testing.T) {
		t.Parallel()
		r := newRefuseInsteadRig(t)
		// Exit 0 with a --- FAIL line still fails: the line, not only the exit code, decides.
		out, errs, code := r.run(t, "refuse", "echo '--- FAIL: TestBeta (0.01s)'")
		if code != 12 {
			t.Fatalf("exit %d, want 12\nstdout:\n%s\nstderr:\n%s", code, out, errs)
		}
		if want := "oracle-refuse-instead: refusals do not cure TestBeta\n"; out != want {
			t.Errorf("stdout = %q, want %q", out, want)
		}
		log := readText(t, r.log)
		if !strings.Contains(log, "verify TestAlpha|TestBeta\n") || strings.Count(log, "build\n") != 2 {
			t.Errorf("want two table builds and the failing names as $RX:\n%s", log)
		}
		// The stub did commit a refusal (the verify ran after it); no branch may still reach it.
		if got := r.git(t, "log", "--all", "--format=%s", "--grep=dirac: refuse"); got != "" {
			t.Errorf("the refusal commit must be gone with the branch, still reachable: %q", got)
		}
		r.assertRestored(t)
		if _, err := os.Stat(filepath.Join(r.repo, "dirac/support/stray.txt")); !os.IsNotExist(err) {
			t.Errorf("the untracked refusal leftover must be cleaned: %v", err)
		}
		if got := readText(t, filepath.Join(r.repo, "dirac/support/tracked.txt")); got != "kept\n" {
			t.Errorf("the modified tracked file must be restored: %q", got)
		}
	})

	t.Run("cures-exit-0", func(t *testing.T) {
		t.Parallel()
		r := newRefuseInsteadRig(t)
		out, errs, code := r.run(t, "refuse", "echo ok")
		if code != 0 {
			t.Fatalf("exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out, errs)
		}
		for _, want := range []string{
			"oracle-refuse-instead: refused 1 cards; 2 failing tests pass\n",
			`go run ./dirac/kgen -refuse "Card A" -why "gorge follows Rule: CR 601.2; dirac not yet"` + "\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout lacks %q:\n%s", want, out)
			}
		}
		if got := r.git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != refuseInsteadBranch {
			t.Errorf("HEAD is on %q, want %s", got, refuseInsteadBranch)
		}
		if got, want := r.git(t, "log", "--format=%s", r.head+".."+refuseInsteadBranch),
			"oracle: sentinel the tests #4559 reddened\ndirac: refuse 1 cards the rules fix diverges on"; got != want {
			t.Errorf("branch commits (newest first):\n%s\nwant:\n%s", got, want)
		}
		got := r.git(t, "show", refuseInsteadBranch+":dirac/oracle/testdata/sentinels.txt")
		if want := "# sentinels\nTestOld\n\n# Named by oracle run 45740776172 (auto-refusal for #4559)\nTestAlpha"; got != want {
			t.Errorf("sentinels.txt on the branch:\n%s\nwant:\n%s", got, want)
		}
	})
}
