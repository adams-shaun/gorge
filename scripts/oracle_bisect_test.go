package scripts

// scripts/oracle-bisect.sh on a tiny repo: first-parent merges, a stubbed test command, the culprit named, and
// the ENV (exit 125) path.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func bisectRepo(t *testing.T) (repo, good, bad, tests string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	script, err := os.ReadFile("oracle-bisect.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	repo = filepath.Join(root, "repo")
	g := func(args ...string) string {
		return sh(t, repo, "git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	}
	write(t, filepath.Join(repo, "scripts", "oracle-bisect.sh"), string(script), 0o755)
	write(t, filepath.Join(repo, "f.txt"), "0\n", 0o644)
	sh(t, root, "git", "init", "-q", "-b", "main", repo)
	g("add", "-A")
	g("commit", "-q", "-m", "base")
	good = strings.TrimSpace(g("rev-parse", "HEAD"))
	// Four PRs, each a branch with two commits merged with --no-ff; PR 103 plants the break.
	for i := 101; i <= 104; i++ {
		br := "pr" + string(rune('0'+i-100))
		g("checkout", "-q", "-b", br, "main")
		if i == 103 {
			write(t, filepath.Join(repo, "broken"), "yes\n", 0o644)
		}
		write(t, filepath.Join(repo, "f.txt"), br+"\n", 0o644)
		g("add", "-A")
		g("commit", "-q", "-m", "work "+br)
		write(t, filepath.Join(repo, "g.txt"), br+"\n", 0o644)
		g("add", "-A")
		g("commit", "-q", "-m", "more "+br)
		g("checkout", "-q", "main")
		g("merge", "-q", "--no-ff", "-m", "Merge pull request #"+string(rune('0'+i/100))+string(rune('0'+i/10%10))+string(rune('0'+i%10))+" from x/"+br, br)
	}
	bad = strings.TrimSpace(g("rev-parse", "HEAD"))
	tests = filepath.Join(root, "tests.txt")
	write(t, tests, "# failing\nTestAlpha\nTestBeta\n", 0o644)
	return repo, good, bad, tests
}

func runBisect(t *testing.T, repo string, env []string, args ...string) (string, int) {
	t.Helper()
	c := exec.Command("bash", append([]string{"scripts/oracle-bisect.sh"}, args...)...)
	c.Dir = repo
	c.Env = append(os.Environ(), env...)
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

const stubTest = `ORACLE_STEP_TEST=if [ -f broken ]; then echo "--- FAIL: TestAlpha (0.00s)"; echo "rx=$RX"; exit 1; fi`

func TestOracleBisectNamesTheCulpritMerge(t *testing.T) {
	t.Parallel()
	repo, good, bad, tests := bisectRepo(t)
	out, code := runBisect(t, repo, []string{"ORACLE_ENV_CHECK=true", "ORACLE_STEP_BUILD=true", stubTest}, good, bad, tests)
	if code != 0 || !strings.Contains(out, "PR 103 ") || !strings.Contains(out, "CULPRIT ") {
		t.Fatalf("want culprit PR 103, got %d\n%s", code, out)
	}
}

func TestOracleBisectStepExits125WithoutCorpusOrTables(t *testing.T) {
	t.Parallel()
	repo, good, _, tests := bisectRepo(t)
	for name, env := range map[string][]string{
		"no corpus":   {"ORACLE_ENV_CHECK=false", "ORACLE_STEP_BUILD=true", stubTest},
		"no tables":   {"ORACLE_ENV_CHECK=true", "ORACLE_STEP_BUILD=false", stubTest},
		"build break": {"ORACLE_ENV_CHECK=true", "ORACLE_STEP_BUILD=true", "ORACLE_STEP_TEST=echo cannot compile; exit 1"},
	} {
		out, code := runBisect(t, repo, env, "--step", tests)
		if code != 125 {
			t.Fatalf("%s: want exit 125, got %d\n%s", name, code, out)
		}
	}
	// A real failure is exit 1 and a pass is exit 0.
	env := []string{"ORACLE_ENV_CHECK=true", "ORACLE_STEP_BUILD=true", stubTest}
	sh(t, repo, "git", "checkout", "-q", good)
	if _, code := runBisect(t, repo, env, "--step", tests); code != 0 {
		t.Fatalf("clean tree: want 0, got %d", code)
	}
	write(t, filepath.Join(repo, "broken"), "x", 0o644)
	if _, code := runBisect(t, repo, env, "--step", tests); code != 1 {
		t.Fatalf("broken tree: want 1, got %d", code)
	}
}

func TestOracleBisectAllStepsSkippedFindsNoCulprit(t *testing.T) {
	t.Parallel()
	repo, good, bad, tests := bisectRepo(t)
	out, code := runBisect(t, repo, []string{"ORACLE_ENV_CHECK=false", stubTest}, good, bad, tests)
	if code != 3 || strings.Contains(out, "CULPRIT") {
		t.Fatalf("want exit 3 and no culprit, got %d\n%s", code, out)
	}
}

// TestOracleBisectDefaultTestCommandStartsUnderSetU: with no ORACLE_STEP_TEST the script's default test
// command must not expand $RX at startup (set -u): it did, and every CI bisect died "RX: unbound variable"
// before its first step (oracle run 45297496438). An empty hook value selects the default.
func TestOracleBisectDefaultTestCommandStartsUnderSetU(t *testing.T) {
	t.Parallel()
	repo, good, bad, tests := bisectRepo(t)
	env := []string{"ORACLE_STEP_TEST=", "ORACLE_ENV_CHECK=false", "ORACLE_STEP_BUILD=true"}
	if out, code := runBisect(t, repo, env, "--step", tests); code != 125 {
		t.Fatalf("--step without corpus: want 125, got %d\n%s", code, out)
	}
	if out, code := runBisect(t, repo, env, good, bad, tests); code != 3 || strings.Contains(out, "unbound") {
		t.Fatalf("all steps skipped: want exit 3, got %d\n%s", code, out)
	}
}
