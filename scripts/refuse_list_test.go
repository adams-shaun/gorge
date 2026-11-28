package scripts

// TestRefuseList drives scripts/refuse-list.sh on the fixtures under
// testdata/refuse-list: one kgen -refuse line per distinct card a bad game names (the
// first implicated card, else the first recent source), at the lowest pair that chose
// it; exit 3 when a bad game names no card; --apply runs the same lines through
// REFUSE_LIST_RUN instead of printing them.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const refuseListScript = "refuse-list.sh"

func refuseListFixture(name string) string {
	return filepath.Join("testdata", "refuse-list", name)
}

// refuseListRun runs the script with args and extra env, and returns stdout, stderr and
// the exit code.
func refuseListRun(t *testing.T, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{refuseListScript}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		code = ee.ExitCode()
	case err != nil:
		t.Fatalf("%s: %v", refuseListScript, err)
	}
	return out.String(), errb.String(), code
}

func refuseListLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func refuseListLastLine(s string) string {
	l := refuseListLines(s)
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1]
}

// refuseListRecorder writes a runner that appends its last argument, one a line, to the
// returned log, and exits with code.
func refuseListRecorder(t *testing.T, failOn string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "ran.log")
	rec := filepath.Join(dir, "rec.sh")
	body := "#!/bin/sh\nfor a; do last=$a; done\nprintf '%s\\n' \"$last\" >> \"" + log + "\"\n"
	if failOn != "" {
		body += "case \"$last\" in *'" + failOn + "'*) exit 7;; esac\n"
	}
	if err := os.WriteFile(rec, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return rec, log
}

func TestRefuseList(t *testing.T) {
	t.Run("two-games", func(t *testing.T) {
		out, errs, code := refuseListRun(t, nil, "fdn", "autopay", refuseListFixture("two-games.log"))
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		want := `go run ./dirac/cmd/kgen -refuse "Card A" -why "diverges from gorge at fdn autopay pair 3" -repro fdn:autopay:3`
		if got := refuseListLines(out); len(got) != 1 || got[0] != want {
			t.Fatalf("stdout = %q, want exactly\n%s", got, want)
		}
		if got := refuseListLastLine(errs); got != "refuse-list: 2 bad games, 1 cards" {
			t.Fatalf("last stderr line = %q", got)
		}
	})

	t.Run("recent-only", func(t *testing.T) {
		out, errs, code := refuseListRun(t, nil, "fdn", "autopay", refuseListFixture("recent-only.log"))
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		want := `go run ./dirac/cmd/kgen -refuse "Card C" -why "diverges from gorge at fdn autopay pair 4" -repro fdn:autopay:4`
		if got := refuseListLines(out); len(got) != 1 || got[0] != want {
			t.Fatalf("stdout = %q, want exactly\n%s", got, want)
		}
	})

	t.Run("none", func(t *testing.T) {
		out, errs, code := refuseListRun(t, nil, "fdn", "autopay", refuseListFixture("none.log"))
		if code != 0 || out != "" {
			t.Fatalf("exit %d, stdout %q; want 0 and empty", code, out)
		}
		if got := refuseListLastLine(errs); got != "refuse-list: no bad game in the input" {
			t.Fatalf("last stderr line = %q", got)
		}
	})

	t.Run("unnamed", func(t *testing.T) {
		_, errs, code := refuseListRun(t, nil, "fdn", "autopay", refuseListFixture("unnamed.log"))
		if code != 3 {
			t.Fatalf("exit %d, want 3; stderr:\n%s", code, errs)
		}
		if !strings.Contains(errs, "refuse-list: pair 9: no card named\n") {
			t.Fatalf("stderr lacks the no-card line:\n%s", errs)
		}
	})

	t.Run("quote", func(t *testing.T) {
		const card = `The "Quoted" \ Card`
		const why = `it's "odd" \ $HOME`
		out, errs, code := refuseListRun(t, nil, "--why", why, "fdn", "autopay", refuseListFixture("quote.log"))
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		lines := refuseListLines(out)
		if len(lines) != 1 || !strings.Contains(lines[0], `-refuse "The \"Quoted\" \\ Card"`) {
			t.Fatalf("stdout = %q, want the name escaped", lines)
		}
		const prefix = "go run ./dirac/cmd/kgen "
		if !strings.HasPrefix(lines[0], prefix) {
			t.Fatalf("line %q lacks %q", lines[0], prefix)
		}
		// Swap kgen for printf and let bash parse the line: every argument comes back
		// as written.
		rt, err := exec.Command("bash", "-c", `printf '%s\n' `+strings.TrimPrefix(lines[0], prefix)).Output()
		if err != nil {
			t.Fatalf("bash -c: %v", err)
		}
		want := []string{"-refuse", card, "-why", why, "-repro", "fdn:autopay:2"}
		if got := refuseListLines(string(rt)); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("round trip = %q, want %q", got, want)
		}
	})

	t.Run("apply-runs-in-order", func(t *testing.T) {
		logs := []string{refuseListFixture("recent-only.log"), refuseListFixture("two-games.log")}
		dry, _, code := refuseListRun(t, nil, append([]string{"fdn", "autopay"}, logs...)...)
		if code != 0 || len(refuseListLines(dry)) != 2 {
			t.Fatalf("dry run: exit %d, stdout %q; want two lines", code, dry)
		}
		rec, log := refuseListRecorder(t, "")
		out, errs, code := refuseListRun(t, []string{"REFUSE_LIST_RUN=" + rec}, append([]string{"--apply", "fdn", "autopay"}, logs...)...)
		if code != 0 {
			t.Fatalf("--apply: exit %d, stderr:\n%s", code, errs)
		}
		if out != "" {
			t.Fatalf("--apply printed %q", out)
		}
		ran, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		if string(ran) != dry {
			t.Fatalf("ran:\n%s\nwant the dry run:\n%s", ran, dry)
		}
	})

	t.Run("apply-stops-at-first-failure", func(t *testing.T) {
		logs := []string{refuseListFixture("recent-only.log"), refuseListFixture("two-games.log")}
		rec, log := refuseListRecorder(t, `"Card A"`)
		_, errs, code := refuseListRun(t, []string{"REFUSE_LIST_RUN=" + rec}, append([]string{"--apply", "fdn", "autopay"}, logs...)...)
		if code != 7 {
			t.Fatalf("--apply: exit %d, want the runner's 7; stderr:\n%s", code, errs)
		}
		ran, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		if got := refuseListLines(string(ran)); len(got) != 1 || !strings.Contains(got[0], `"Card A"`) {
			t.Fatalf("ran %q, want only the Card A line (sorted first, failed, stopped)", got)
		}
	})

	t.Run("usage", func(t *testing.T) {
		if _, _, code := refuseListRun(t, nil); code != 2 {
			t.Fatalf("no arguments: exit %d, want 2", code)
		}
	})
}
