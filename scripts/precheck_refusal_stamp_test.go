package scripts

// TestPrecheckDoesNotStampRefusalCommit runs scripts/pr-precheck.sh on a throwaway
// repository with the real refuse-on-red.sh in apply mode: the refusal commit it makes
// after the checks must get no pass stamp, and a second run must check it.

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// precheckRefusalRepo is a refuse-on-red fixture repository that can also run
// pr-precheck.sh: its base commit holds that script and stand-ins for the helpers.
func precheckRefusalRepo(t *testing.T) *refuseOnRedRepo {
	t.Helper()
	script, err := os.ReadFile("pr-precheck.sh")
	if err != nil {
		t.Fatal(err)
	}
	r := newRefuseOnRedRepo(t, refuseOnRedChange{
		baseFiles: [][2]string{
			{"scripts/pr-precheck.sh", string(script)},
			{"scripts/pr-cards.sh", ""},
			{"scripts/pr-derived.sh", "#!/bin/sh\nexit 0\n"},
			{"scripts/refuse-on-red.sh", "#!/bin/sh\nexec bash \"" + refuseOnRedAbs(t) + "\" \"$@\"\n"},
		},
		files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
		message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
	})
	if err := os.Chmod(filepath.Join(r.dir, "scripts", "refuse-on-red.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.git(t, "commit", "-q", "-a", "--amend", "--no-edit")
	return r
}

// precheckRefusalTools puts a `go` that records its arguments and passes, and a
// `systemd-run` that only runs the command, first on PATH. It returns the PATH
// assignment and the go call log.
func precheckRefusalTools(t *testing.T) (string, string) {
	t.Helper()
	bin := t.TempDir()
	calls := filepath.Join(bin, "go-calls.log")
	for name, body := range map[string]string{
		"go":          "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"" + calls + "\"\n",
		"systemd-run": "#!/bin/sh\nwhile [ \"$#\" -gt 0 ] && [ \"$1\" != env ]; do shift; done\nexec \"$@\"\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return "PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), calls
}

// precheckRefusalStamps reads the two pass stamps: the git-dir one ("" when absent)
// and the names of the diff-keyed ones.
func precheckRefusalStamps(t *testing.T, r *refuseOnRedRepo, stampDir string) (string, []string) {
	t.Helper()
	head, err := os.ReadFile(filepath.Join(r.dir, ".git", "pr-precheck-ok"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(stampDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var keys []string
	for _, e := range entries {
		keys = append(keys, e.Name())
	}
	return strings.TrimSpace(string(head)), keys
}

func precheckRefusalDiffKey(t *testing.T, r *refuseOnRedRepo) string {
	t.Helper()
	cmd := exec.Command("git", "diff", "--binary", r.base+"...HEAD")
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(out))
}

func TestPrecheckDoesNotStampRefusalCommit(t *testing.T) {
	run := func(t *testing.T, r *refuseOnRedRepo, env []string) (string, int) {
		t.Helper()
		cmd := exec.Command("bash", "scripts/pr-precheck.sh", r.base)
		cmd.Dir = r.dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"PR_PRECHECK_FORCE=", "PR_PRECHECK_BRANCH=wt/fixture")
		cmd.Env = append(cmd.Env, refuseOnRedIdentity...)
		cmd.Env = append(cmd.Env, env...)
		out, err := cmd.CombinedOutput()
		var ee *exec.ExitError
		switch {
		case errors.As(err, &ee):
			return string(out), ee.ExitCode()
		case err != nil:
			t.Fatalf("run pr-precheck.sh: %v", err)
		}
		return string(out), 0
	}
	// setup returns a repository with one diverging card and the env of a run on it.
	setup := func(t *testing.T, mode string) (*refuseOnRedRepo, []string, string, string) {
		t.Helper()
		r := precheckRefusalRepo(t)
		logs := refuseOnRedCleanLogs()
		logs["fdn-autopay"] = refuseOnRedBadGame
		rec, _ := refuseOnRedRecorder(t, r)
		path, calls := precheckRefusalTools(t)
		stampDir := filepath.Join(t.TempDir(), "stamps")
		env := append(refuseOnRedEnv(refuseOnRedStub(t, logs)),
			"REFUSE_ON_RED="+mode, "REFUSE_LIST_RUN="+rec, "PRECHECK_STAMP_DIR="+stampDir, path)
		if head, keys := precheckRefusalStamps(t, r, stampDir); head != "" || len(keys) != 0 {
			t.Fatalf("precondition: stamps before the first run: %q, %q", head, keys)
		}
		return r, env, stampDir, calls
	}

	t.Run("apply", func(t *testing.T) {
		r, env, stampDir, calls := setup(t, "apply")
		checked := r.git(t, "rev-parse", "HEAD")

		out, code := run(t, r, env)
		refusal := r.git(t, "rev-parse", "HEAD")
		if refusal == checked || !strings.Contains(out, "refuse-on-red: refused 1 cards\n") {
			t.Fatalf("precondition: the first run made no refusal commit (exit %d):\n%s", code, out)
		}
		if code != 1 {
			t.Errorf("first run exit %d, want 1:\n%s", code, out)
		}
		want := "pr-precheck: refuse-on-red committed refusals that no check has seen: run scripts/pr-precheck.sh " +
			r.base + " again"
		if got := refuseOnRedLast(out); got != want {
			t.Errorf("first run verdict = %q, want %q", got, want)
		}
		if head, keys := precheckRefusalStamps(t, r, stampDir); head != "" || len(keys) != 0 {
			t.Errorf("first run stamped an unchecked commit: git-dir %q, diff-keyed %q (checked %s, refusal %s)",
				head, keys, checked, refusal)
		}

		// The second run checks the refusal commit: it touches dirac/, so the support
		// ledger step runs, which the first run's diff did not reach.
		const ledger = "./dirac/kgen"
		first, err := os.ReadFile(calls)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(first), ledger) {
			t.Fatalf("precondition: the first run already ran the support ledger:\n%s", first)
		}
		out, code = run(t, r, env)
		if code != 0 || refuseOnRedLast(out) != "pr-precheck: ok" {
			t.Fatalf("second run exit %d:\n%s", code, out)
		}
		if head := r.git(t, "rev-parse", "HEAD"); head != refusal {
			t.Fatalf("second run moved HEAD from %s to %s", refusal, head)
		}
		all, err := os.ReadFile(calls)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.TrimPrefix(string(all), string(first)), ledger) {
			t.Fatalf("second run did not check the refusal commit; go calls:\n%s", all)
		}
		head, keys := precheckRefusalStamps(t, r, stampDir)
		if key := precheckRefusalDiffKey(t, r); head != refusal || len(keys) != 1 || keys[0] != key {
			t.Fatalf("second run stamps: git-dir %q, diff-keyed %q; want %q and [%q]", head, keys, refusal, key)
		}
	})

	// Report mode commits nothing, so one run still passes and writes both stamps.
	t.Run("report", func(t *testing.T) {
		r, env, stampDir, _ := setup(t, "report")
		checked := r.git(t, "rev-parse", "HEAD")
		out, code := run(t, r, env)
		if !strings.Contains(out, "refuse-on-red: would refuse 1 cards (report-only):\n") {
			t.Fatalf("precondition: refuse-on-red did not report the card (exit %d):\n%s", code, out)
		}
		if code != 0 || refuseOnRedLast(out) != "pr-precheck: ok" {
			t.Fatalf("report run exit %d:\n%s", code, out)
		}
		head, keys := precheckRefusalStamps(t, r, stampDir)
		if key := precheckRefusalDiffKey(t, r); head != checked || len(keys) != 1 || keys[0] != key {
			t.Fatalf("report run stamps: git-dir %q, diff-keyed %q; want %q and [%q]", head, keys, checked, key)
		}
	})
}
