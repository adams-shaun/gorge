package oracle

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// isolatedEnv marks a child test process started by isolate.
const isolatedEnv = "KORACLE_ISOLATED"

// isolate runs the calling top-level test in a child process of this test
// binary, in parallel with the package's other parallel tests. It is for the
// heavy tests that set package globals the games read (dirac.Debug* hooks
// and stats, OccupancyHook): in process they would race every other game, so
// they would have to run serially; a child process has its own globals, so
// the test plays exactly the games it plays serially, alone.
//
// Use it as the test's first statement: `if isolate(t) { return }`. It
// returns true in the parent, after reporting the child's result (pass,
// fail with its output, or skip); in the child it returns false and the test
// body runs.
func isolate(t *testing.T) bool {
	t.Helper()
	if os.Getenv(isolatedEnv) != "" {
		return false
	}
	t.Parallel()
	args := []string{"-test.run=^" + regexp.QuoteMeta(t.Name()) + "$", "-test.v"}
	if d, ok := t.Deadline(); ok {
		args = append(args, "-test.timeout="+time.Until(d).Round(time.Second).String())
	}
	if testing.Short() {
		args = append(args, "-test.short")
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), isolatedEnv+"=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	text := out.String()
	switch {
	case err != nil:
		t.Fatalf("isolated %s: %v\n%s", t.Name(), err, text)
	case strings.Contains(text, "--- SKIP: "+t.Name()+" "):
		t.Skip("isolated:\n" + text)
	case !strings.Contains(text, "--- PASS: "+t.Name()+" "):
		t.Fatalf("isolated %s: no PASS line\n%s", t.Name(), text)
	case testing.Verbose():
		t.Log("isolated:\n" + text)
	}
	return true
}
