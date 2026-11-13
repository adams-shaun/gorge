package oracle

import (
	"bytes"
	"encoding/json"
	"flag"
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
	if isolatedChild() {
		return false
	}
	t.Parallel()
	runIsolated(t)
	return true
}

// isolatedChild reports whether this process is an isolate child: a test
// whose subtests isolate skips its own whole-run checks there, since the
// child plays only the one subtest.
func isolatedChild() bool { return os.Getenv(isolatedEnv) != "" }

// tallyPrefix marks the line a child's isolated subtest logs its tally on.
const tallyPrefix = "isolate-tally: "

// isolateTally is isolate for one parallel subtest of a test that sums its
// subtests' tallies: in the parent it runs the subtest in a child process,
// decodes the tally the child logged (logTally) into v and returns true; in
// the child it returns false, and the subtest plays and ends with
// logTally(t, v). It does not call t.Parallel (the subtest already has).
func isolateTally(t *testing.T, v any) bool {
	t.Helper()
	if isolatedChild() {
		return false
	}
	text := runIsolated(t)
	if t.Skipped() {
		return true
	}
	i := strings.Index(text, tallyPrefix)
	if i < 0 {
		t.Fatalf("isolated %s: no tally line\n%s", t.Name(), text)
	}
	line := text[i+len(tallyPrefix):]
	if j := strings.IndexByte(line, '\n'); j >= 0 {
		line = line[:j]
	}
	if err := json.Unmarshal([]byte(line), v); err != nil {
		t.Fatalf("isolated %s: tally: %v\n%s", t.Name(), err, text)
	}
	return true
}

// logTally logs v (exported fields, JSON) for isolateTally's parent.
func logTally(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(tallyPrefix + string(b))
}

// runIsolated runs t (a test or a subtest) alone in a child process of this
// test binary, reports its result and returns its output.
func runIsolated(t *testing.T) string {
	t.Helper()
	// one anchored pattern per level, so a subtest isolates too
	levels := strings.Split(t.Name(), "/")
	for i, l := range levels {
		levels[i] = "^" + regexp.QuoteMeta(l) + "$"
	}
	args := []string{"-test.run=" + strings.Join(levels, "/"), "-test.v"}
	// the child's parallel subtests share the parent's per-process bound
	if f := flag.Lookup("test.parallel"); f != nil {
		args = append(args, "-test.parallel="+f.Value.String())
	}
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
	return text
}
