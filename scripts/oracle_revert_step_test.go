package scripts

// oracle.yml's "Open a draft revert PR" step, run as text against a local repo, a bare origin and a stubbed gh: a
// second bisect blaming the same PR must comment on the open revert PR or force-update the bot branch, never fail
// on a rejected plain push (oracle run 45740776172: the push of hotfix/auto-revert-4559 was non-fast-forward, the
// branch was left by the earlier bisect behind closed draft PR #5039, and no revert PR was opened).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// revertStep extracts the run block of the "Open a draft revert PR" step from oracle.yml: the test always runs the
// real workflow text, not a copy that can drift.
func revertStep(t *testing.T) string {
	t.Helper()
	or := readText(t, "../.github/workflows/oracle.yml")
	const step = "Open a draft revert PR"
	i := strings.Index(or, step)
	if i < 0 {
		t.Fatalf("oracle.yml has no %q step", step)
	}
	rest := or[i:]
	j := strings.Index(rest, "run: |")
	if j < 0 {
		t.Fatalf("the %q step has no run block", step)
	}
	rest = rest[j+len("run: |"):]
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(rest, "\n"), "\n") {
		// A step-level "- name:" (the block is the job's last today) ends the run block.
		if strings.HasPrefix(ln, "      - ") {
			break
		}
		lines = append(lines, ln)
	}
	indent := -1
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if n := len(ln) - len(strings.TrimLeft(ln, " ")); indent < 0 || n < indent {
			indent = n
		}
	}
	if indent <= 0 {
		t.Fatalf("the %q run block is not indented", step)
	}
	for k, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			lines[k] = ln[indent:]
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// revertStepRepo builds a main history with two first-parent merges and a bare origin whose bot branch
// hotfix/auto-revert-4559 was left behind by an earlier bisect: its tip reverts #4500, a commit that is not on
// main, so a plain push of a fresh tip is rejected as non-fast-forward — the run 45740776172 failure.
func revertStepRepo(t *testing.T) (repo, culprit, head string) {
	t.Helper()
	return revertStepRepoWith(t, nil)
}

// revertStepRepoWith is revertStepRepo with extra files (path -> content) in main's base commit.
func revertStepRepoWith(t *testing.T, extra map[string]string) (repo, culprit, head string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	repo = filepath.Join(root, "repo")
	g := func(args ...string) string {
		return sh(t, repo, "git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	}
	write(t, filepath.Join(repo, "f.txt"), "0\n", 0o644)
	for p, c := range extra {
		write(t, filepath.Join(repo, p), c, 0o644)
	}
	sh(t, root, "git", "init", "-q", "-b", "main", repo)
	g("add", "-A")
	g("commit", "-q", "-m", "base")
	g("checkout", "-q", "-b", "pr4500", "main")
	write(t, filepath.Join(repo, "a.txt"), "4500\n", 0o644)
	g("add", "-A")
	g("commit", "-q", "-m", "work 4500")
	g("checkout", "-q", "main")
	m4500 := strings.TrimSpace(g("rev-parse", "HEAD"))
	g("checkout", "-q", "-b", "pr4559", "main")
	write(t, filepath.Join(repo, "b.txt"), "4559\n", 0o644)
	g("add", "-A")
	g("commit", "-q", "-m", "work 4559")
	g("checkout", "-q", "main")
	g("merge", "-q", "--no-ff", "-m", "Merge pull request #4559 from x/pr4559", "pr4559")
	culprit = strings.TrimSpace(g("rev-parse", "HEAD"))
	write(t, filepath.Join(repo, "c.txt"), "later\n", 0o644)
	g("add", "-A")
	g("commit", "-q", "-m", "later work on main")
	head = strings.TrimSpace(g("rev-parse", "HEAD"))
	origin := filepath.Join(root, "origin.git")
	sh(t, root, "git", "init", "-q", "--bare", "-b", "main", origin)
	g("remote", "add", "origin", origin)
	g("push", "-q", "origin", "main")
	// The earlier bisect's leftover: revert #4500 from an older main and push it as the bot branch for #4559.
	g("checkout", "-q", "-b", "stale", m4500)
	g("revert", "--no-edit", "-m", "1", m4500)
	g("push", "-q", "origin", "stale:hotfix/auto-revert-4559")
	g("checkout", "-q", "main")
	g("branch", "-q", "-D", "stale")
	write(t, filepath.Join(repo, "failing.txt"), "TestAlpha\nTestBeta\n", 0o644)
	return repo, culprit, head
}

// runRevertStep runs the extracted step text as Actions would (bash -e -o pipefail) with gh stubbed to log its
// commands and bodies, and, when STUB_OPEN_PR is set, to answer pr list with that open PR number. culprit and head
// are the real shas of the temp repo: the step really reverts and switches on them.
func runRevertStep(t *testing.T, repo, script, culprit, head, openPR string) (log string, code int) {
	t.Helper()
	root := filepath.Dir(repo)
	bin := filepath.Join(root, "bin")
	write(t, filepath.Join(bin, "gh"), `#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
prev=""
for a in "$@"; do
  if [ "$prev" = "--body-file" ]; then echo "--- body $a" >> "$GH_LOG"; cat "$a" >> "$GH_LOG"; fi
  prev=$a
done
case "$1 $2" in
  "pr list")
    if [ -n "${STUB_OPEN_PR:-}" ]; then printf '[{"number": %s}]\n' "$STUB_OPEN_PR"; else echo "[]"; fi ;;
esac
`, 0o755)
	ghLog := filepath.Join(root, "gh.log")
	write(t, filepath.Join(repo, "step.sh"), script, 0o755)
	c := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "step.sh")
	c.Dir = repo
	path, _ := os.LookupEnv("PATH")
	env := map[string]string{
		"PATH":              bin + string(os.PathListSeparator) + path,
		"PR":                "4559",
		"CULPRIT":           culprit,
		"GITHUB_SHA":        head,
		"GITHUB_REPOSITORY": "adams-shaun/gorge",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_RUN_ID":     "45740776172",
		"GH_TOKEN":          "t",
		"GH_LOG":            ghLog,
		"STUB_OPEN_PR":      openPR,
	}
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}
	out, err := c.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(ghLog)
	if err != nil {
		t.Fatalf("gh stub wrote no log (%d): %v\n%s", code, err, out)
	}
	return string(b) + string(out), code
}

// revertStepBody returns the body of the last --body-file the gh stub logged: the text after its
// "--- body <path>" marker, read from gh.log alone. The step's stdout is not included, so an assertion here
// cannot pass on the "| tee sentinels.out" output instead of the PR body.
func revertStepBody(t *testing.T, repo string) string {
	t.Helper()
	ghLog := readText(t, filepath.Join(filepath.Dir(repo), "gh.log"))
	i := strings.LastIndex(ghLog, "--- body ")
	if i < 0 {
		t.Fatalf("gh stub logged no --body-file body:\n%s", ghLog)
	}
	nl := strings.IndexByte(ghLog[i:], '\n')
	if nl < 0 {
		t.Fatalf("gh stub's body marker has no body:\n%s", ghLog)
	}
	return ghLog[i+nl+1:]
}

// originTip returns the sha origin has for ref, or "" when it has none.
func originTip(t *testing.T, repo, ref string) string {
	t.Helper()
	return strings.TrimSpace(sh(t, repo, "git", "ls-remote", "origin", "refs/heads/"+ref))
}

func TestOracleBisectRevertStep(t *testing.T) {
	t.Parallel()
	step := revertStep(t)

	t.Run("open PR gets a comment, not a second PR or a push", func(t *testing.T) {
		t.Parallel()
		repo, culprit, head := revertStepRepo(t)
		before := originTip(t, repo, "hotfix/auto-revert-4559")
		log, code := runRevertStep(t, repo, step, culprit, head, "5039")
		if code != 0 {
			t.Fatalf("an already-handled culprit must not fail the job: got %d\n%s", code, log)
		}
		if !strings.Contains(log, "gh pr list --head hotfix/auto-revert-4559 --state open --json number") {
			t.Errorf("the step must look for an open PR on the bot branch:\n%s", log)
		}
		if !strings.Contains(log, "gh pr comment 5039") {
			t.Errorf("the open revert PR must get a comment:\n%s", log)
		}
		if strings.Contains(log, "gh pr create") {
			t.Errorf("a second PR for the same head must not be opened:\n%s", log)
		}
		if after := originTip(t, repo, "hotfix/auto-revert-4559"); after != before {
			t.Errorf("the bot branch must not move when its PR is open: %s -> %s", before, after)
		}
		// The comment carries the run link, the failing tests and the signature.
		for _, want := range []string{
			"https://github.com/adams-shaun/gorge/actions/runs/45740776172",
			"- TestAlpha", "- TestBeta",
			"This Robot (gorge agentctl swarm) Shan't Be Blamed For Stupid Humans",
			culprit, head,
		} {
			if !strings.Contains(log, want) {
				t.Errorf("the comment lacks %q:\n%s", want, log)
			}
		}
	})

	t.Run("no open PR force-updates the bot branch and opens one draft PR", func(t *testing.T) {
		t.Parallel()
		repo, culprit, head := revertStepRepo(t)
		log, code := runRevertStep(t, repo, step, culprit, head, "")
		if code != 0 {
			t.Fatalf("want exit 0, got %d\n%s", code, log)
		}
		if !strings.Contains(log, "gh pr create --draft") {
			t.Errorf("a fresh culprit wants a draft PR:\n%s", log)
		}
		if strings.Contains(log, "gh pr comment") {
			t.Errorf("with no open PR there is nothing to comment on:\n%s", log)
		}
		// The leftover branch was force-updated to the new revert, and the PR carries the failing tests.
		local := strings.TrimSpace(sh(t, repo, "git", "rev-parse", "hotfix/auto-revert-4559"))
		remote := originTip(t, repo, "hotfix/auto-revert-4559")
		if remote == "" || !strings.Contains(remote, local) {
			t.Errorf("origin's bot branch was not updated to the new revert %s: %s", local, remote)
		}
		for _, want := range []string{
			"- TestAlpha", "- TestBeta",
			"This Robot (gorge agentctl swarm) Shan't Be Blamed For Stupid Humans",
		} {
			if !strings.Contains(log, want) {
				t.Errorf("the PR body lacks %q:\n%s", want, log)
			}
		}
	})

	t.Run("revert branch carries a sentinel commit", func(t *testing.T) {
		t.Parallel()
		script := readText(t, "sentinel-append.sh")
		const base = "# sentinels\nTestOld\n"
		repo, culprit, head := revertStepRepoWith(t, map[string]string{
			"scripts/sentinel-append.sh":                 script,
			"dirac/oracle/testdata/sentinels.txt":        base,
			"dirac/oracle/testdata/ci-shard-seconds.txt": "# <test> <seconds>\n",
			"dirac/oracle/a_test.go":                     "package oracle\n\nimport \"testing\"\n\nfunc TestOld(t *testing.T) {}\n\nfunc TestAlpha(t *testing.T) {}\n",
		})
		log, code := runRevertStep(t, repo, step, culprit, head, "")
		if code != 0 {
			t.Fatalf("want exit 0, got %d\n%s", code, log)
		}
		br := "hotfix/auto-revert-4559"
		tip := originTip(t, repo, br)
		if tip == "" {
			t.Fatalf("origin has no %s", br)
		}
		tip = strings.Fields(tip)[0]
		if subj := strings.TrimSpace(sh(t, repo, "git", "log", "-1", "--format=%s", tip)); subj != "oracle: sentinel the tests #4559 reddened" {
			t.Errorf("tip subject %q", subj)
		}
		parent := strings.TrimSpace(sh(t, repo, "git", "rev-parse", tip+"^"))
		if subj := strings.TrimSpace(sh(t, repo, "git", "log", "-1", "--format=%s", parent)); !strings.HasPrefix(subj, "Revert ") {
			t.Errorf("tip's parent is not the revert: %q", subj)
		}
		if p := strings.TrimSpace(sh(t, repo, "git", "rev-parse", parent+"^")); p != head {
			t.Errorf("the revert's parent is %s, want main's head %s", p, head)
		}
		got := sh(t, repo, "git", "show", tip+":dirac/oracle/testdata/sentinels.txt")
		if want := base + "\n# Named by oracle run 45740776172 (auto-revert of #4559)\nTestAlpha\n"; got != want {
			t.Errorf("sentinels.txt on the bot branch:\n%s\nwant:\n%s", got, want)
		}
		if strings.Contains(got, "TestBeta") {
			t.Errorf("TestBeta is not declared and must not be listed:\n%s", got)
		}
		if !strings.Contains(log, "gh pr create --draft") {
			t.Errorf("the step must open a draft PR:\n%s", log)
		}
		body := revertStepBody(t, repo)
		i := strings.Index(body, "sentinel-append: added 1 of 2")
		if i < 0 {
			t.Errorf("the PR body must carry the sentinel-append report:\n%s", body)
		}
		footer := strings.Index(body, "\n---\n")
		if footer < 0 {
			t.Errorf("the PR body has no `---` footer:\n%s", body)
		} else if i > footer {
			t.Errorf("the sentinel-append report must come before the body's `---` footer:\n%s", body)
		}
	})
}
