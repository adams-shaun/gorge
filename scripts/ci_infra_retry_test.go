package scripts

// scripts/ci-infra-retry.sh runs a CI step a second time when its log names an infrastructure fault
// (scripts/ci-infra-signatures.txt) and carries no test verdict. The script runs here on stub commands; the wiring into
// ci.yml is read as text.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// stubAttempt is what one call of a stub command prints (to stderr, so the log also shows the streams are merged) and
// exits with.
type stubAttempt struct {
	out string
	rc  int
}

// writeStub writes a command whose first call behaves as first and every later call as second. The returned function
// counts the calls.
func writeStub(t *testing.T, dir string, first, second stubAttempt) (string, func() int) {
	t.Helper()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("out1", first.out)
	write("out2", second.out)
	write("stub.sh", `d=$(dirname "$0")
echo x >>"$d/calls"
n=2 rc=`+strconv.Itoa(second.rc)+`
if [ "$(wc -l <"$d/calls")" -eq 1 ]; then n=1 rc=`+strconv.Itoa(first.rc)+`; fi
cat "$d/out$n" >&2
exit $rc
`)
	calls := func() int {
		b, err := os.ReadFile(filepath.Join(dir, "calls"))
		if errors.Is(err, os.ErrNotExist) {
			return 0
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(b), "\n")
	}
	return filepath.Join(dir, "stub.sh"), calls
}

// runInfraRetry runs the script with CI_INFRA_RETRY unset unless env sets it.
func runInfraRetry(t *testing.T, env []string, args ...string) (stdout, stderr string, rc int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"ci-infra-retry.sh"}, args...)...)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CI_INFRA_RETRY=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, env...)
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatal(err)
	}
	return o.String(), e.String(), cmd.ProcessState.ExitCode()
}

func TestCIInfraRetry(t *testing.T) {
	t.Parallel()
	// Read the files under test, so go's test cache sees a change to either.
	readText(t, "ci-infra-retry.sh")
	readText(t, "ci-infra-signatures.txt")

	pass := stubAttempt{"ok  \texample/pkg\t1.0s\n", 0}
	killed := stubAttempt{"signal: killed\nFAIL\texample/pkg\t9.0s\n", 1}
	// killedWith is a log that would retry on its signature, plus the line that must forbid it.
	killedWith := func(line string) stubAttempt { return stubAttempt{line + "\n" + killed.out, 1} }

	for _, tc := range []struct {
		name          string
		first, second stubAttempt
		env           []string
		label         string // "" passes no --name
		rc, runs      int
		warn          string // "" for no retry
	}{
		{name: "pass-runs-once", first: pass, second: killed, label: "stub", rc: 0, runs: 1},
		{name: "killed-then-pass-retries", first: killed, second: pass, label: "test shard 3", rc: 0, runs: 2,
			warn: "::warning::ci-infra-retry: test shard 3 hit killed (exit 1); retrying once"},
		// No --name: the label is the command's first word.
		{name: "killed-twice-fails", first: killed, second: stubAttempt{"exit code 137\n", 7}, rc: 7, runs: 2,
			warn: "::warning::ci-infra-retry: bash hit killed (exit 1); retrying once"},
		// Go's syscall.ENOSPC message is all lowercase, unlike the C tools'.
		{name: "go-no-space-retries", first: stubAttempt{"write /tmp/x: no space left on device\n", 1}, second: pass, label: "stub", rc: 0, runs: 2,
			warn: "::warning::ci-infra-retry: stub hit no-space (exit 1); retrying once"},
		{name: "test-fail-never-retries", first: killedWith("--- FAIL: TestX (0.00s)"), second: pass, label: "stub", rc: 1, runs: 1},
		// Class C: a package binary over its -timeout panics with no --- FAIL, so it retries once; a --- FAIL in the
		// same log is a verdict and forbids the retry, so a real assertion failure reds on attempt 1.
		{name: "test-timeout-retries", first: stubAttempt{"panic: test timed out after 5m0s\n", 1}, second: pass, label: "fresh-clone-check", rc: 0, runs: 2,
			warn: "::warning::ci-infra-retry: fresh-clone-check hit test-timeout (exit 1); retrying once"},
		{name: "test-timeout-with-fail-never-retries", first: stubAttempt{"panic: test timed out after 5m0s\n--- FAIL: TestX (0.00s)\n", 1}, second: pass, label: "fresh-clone-check", rc: 1, runs: 1},
		{name: "bad-game-never-retries", first: killedWith("bad-game\tpool seed=7"), second: pass, label: "stub", rc: 1, runs: 1},
		{name: "build-failed-never-retries", first: killedWith("FAIL\texample/pkg [build failed]"), second: pass, label: "stub", rc: 1, runs: 1},
		{name: "compile-error-never-retries", first: killedWith("pkg/x.go:3:4: undefined: y"), second: pass, label: "stub", rc: 1, runs: 1},
		{name: "unknown-failure-never-retries", first: stubAttempt{"error: something else broke\n", 3}, second: pass, label: "stub", rc: 3, runs: 1},
		{name: "switch-off", first: killed, second: pass, env: []string{"CI_INFRA_RETRY=0"}, label: "stub", rc: 1, runs: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			stub, calls := writeStub(t, dir, tc.first, tc.second)
			log := filepath.Join(dir, "step.log")
			args := []string{"--log", log}
			if tc.label != "" {
				args = append(args, "--name", tc.label)
			}
			stdout, stderr, rc := runInfraRetry(t, tc.env, append(args, "--", "bash", stub)...)
			if rc != tc.rc {
				t.Errorf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", rc, tc.rc, stdout, stderr)
			}
			if n := calls(); n != tc.runs {
				t.Errorf("the command ran %d times, want %d", n, tc.runs)
			}
			if !strings.Contains(stdout, tc.first.out) {
				t.Errorf("attempt 1's output was not shown:\n%s", stdout)
			}
			if tc.warn == "" {
				if strings.Contains(stdout+stderr, "::warning::") {
					t.Errorf("unexpected warning:\n%s%s", stdout, stderr)
				}
				if got := readText(t, log); got != tc.first.out {
					t.Errorf("log = %q, want %q", got, tc.first.out)
				}
				if _, err := os.Stat(log + ".1"); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s.1 must not exist without a retry (stat: %v)", log, err)
				}
				return
			}
			label := tc.label
			if label == "" {
				label = "bash"
			}
			for _, want := range []string{tc.warn, "ci-infra-retry: " + label + " attempt 2 exit " + strconv.Itoa(tc.second.rc)} {
				if !strings.Contains("\n"+stdout, "\n"+want+"\n") {
					t.Errorf("stdout lacks the line %q:\n%s", want, stdout)
				}
			}
			if strings.Count(stdout, "::warning::") != 1 {
				t.Errorf("want exactly one warning:\n%s", stdout)
			}
			if got := readText(t, log+".1"); got != tc.first.out {
				t.Errorf("%s.1 = %q, want attempt 1's %q", log, got, tc.first.out)
			}
			if got := readText(t, log); got != tc.second.out {
				t.Errorf("log = %q, want attempt 2's %q", got, tc.second.out)
			}
		})
	}

	t.Run("usage-exit-2", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		stub, calls := writeStub(t, dir, pass, pass)
		log := filepath.Join(dir, "step.log")
		for what, args := range map[string][]string{
			"no --log":   {"--", "bash", stub},
			"no --":      {"--log", log, "bash", stub},
			"no command": {"--log", log, "--"},
		} {
			stdout, stderr, rc := runInfraRetry(t, nil, args...)
			if rc != 2 || !strings.Contains(stderr, "usage:") {
				t.Errorf("%s: exit %d (want 2), stderr %q (want a usage message), stdout %q", what, rc, stderr, stdout)
			}
		}
		if n := calls(); n != 0 {
			t.Errorf("a usage error ran the command %d times", n)
		}
	})
}

func TestCIInfraRetryWired(t *testing.T) {
	t.Parallel()
	ci := readText(t, "../.github/workflows/ci.yml")
	test := jobBlock(t, ci, "test")
	for _, want := range []string{"bash scripts/ci-infra-retry.sh --log test-shard.log", "bash scripts/ci-test-shard.sh"} {
		if !strings.Contains(test, want) {
			t.Errorf("job test does not run %q", want)
		}
	}
	slices := jobBlock(t, ci, "dirac-slices")
	if !strings.Contains(slices, "bash scripts/ci-infra-retry.sh --log slices.log") {
		t.Error("job dirac-slices does not run its slices through scripts/ci-infra-retry.sh --log slices.log")
	}
	if strings.Contains(slices, "| tee slices.log") {
		t.Error("job dirac-slices still tees slices.log itself: the wrapper owns the log")
	}
	lint := jobBlock(t, ci, "lint")
	if !strings.Contains(lint, "bash scripts/ci-infra-retry.sh --log fresh-clone.log --name fresh-clone-check -- make fresh-clone-check") {
		t.Error("job lint does not run fresh-clone-check through scripts/ci-infra-retry.sh --log fresh-clone.log")
	}
	// The test-timeout signature matches fresh-clone-check.sh's 5m package timeout; keep the two in lockstep.
	if !strings.Contains(readText(t, "fresh-clone-check.sh"), "go test -timeout 5m ./dirac/...") {
		t.Error("fresh-clone-check.sh no longer runs its dirac sweep with `go test -timeout 5m ./dirac/...`: the test-timeout signature will not match")
	}
	body := readText(t, "ci-test-shard.sh")
	for _, want := range []string{"scripts/ci_shard.sh", "ci-splits.tsv", `-skip "^(TestHeads${q:+|$q})\$"`, "$GITHUB_EVENT_NAME"} {
		if !strings.Contains(body, want) {
			t.Errorf("ci-test-shard.sh lacks %q", want)
		}
	}
	if strings.Contains(body, "${{") {
		t.Error("ci-test-shard.sh holds a workflow expression, which only a workflow file expands")
	}
	rows := 0
	for i, line := range strings.Split(readText(t, "ci-infra-signatures.txt"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rows++
		f := strings.Split(line, "\t")
		if len(f) != 2 || f[0] == "" || f[1] == "" {
			t.Errorf("ci-infra-signatures.txt:%d: want name<TAB>ERE, got %q", i+1, line)
			continue
		}
		if _, err := regexp.Compile(f[1]); err != nil {
			t.Errorf("ci-infra-signatures.txt:%d: %v", i+1, err)
		}
	}
	if rows == 0 {
		t.Error("ci-infra-signatures.txt has no rows")
	}
}
