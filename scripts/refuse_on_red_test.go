package scripts

// TestRefuseOnRed drives scripts/refuse-on-red.sh on throwaway git repositories: which
// changes are in scope, what a lockstep failure turns into in report and apply modes,
// and when the script refuses to decide.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const refuseOnRedScript = "refuse-on-red.sh"

const (
	refuseOnRedRule      = "Rule: CR 601.2"
	refuseOnRedEngineGo  = "engine/rules/refuse_on_red_change.go"
	refuseOnRedDiracGo   = "dirac/refuse_on_red_change.go"
	refuseOnRedBadGame   = "pair 0 seed 1 ok\nbad-game\t7\t907\tdivergent\tlife\tCard A|Card B\tCard B|Card A\tCard A|Card B|Plains\nkfuzz: 1 bad-game lines, 0 ok\n"
	refuseOnRedCleanLog  = "pair 0 seed 1 ok\nkfuzz: 0 divergent\n"
	refuseOnRedNoRuleMsg = "fix(rules): keep the card where the rule says\n"
)

func refuseOnRedAbs(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(refuseOnRedScript)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// refuseOnRedRepo is a throwaway repository with a base commit and a branch commit.
type refuseOnRedRepo struct {
	dir  string
	base string
}

type refuseOnRedChange struct {
	files   [][2]string
	message string
}

func refuseOnRedWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *refuseOnRedRepo) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=refuse-on-red", "GIT_AUTHOR_EMAIL=refuse-on-red@example.invalid",
		"GIT_COMMITTER_NAME=refuse-on-red", "GIT_COMMITTER_EMAIL=refuse-on-red@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRefuseOnRedRepo(t *testing.T, change refuseOnRedChange) *refuseOnRedRepo {
	t.Helper()
	r := &refuseOnRedRepo{dir: t.TempDir()}
	r.git(t, "init", "-q")
	refuseOnRedWrite(t, r.dir, "base.txt", "base\n")
	r.git(t, "add", "base.txt")
	r.git(t, "commit", "-q", "-m", "base")
	r.base = r.git(t, "rev-parse", "HEAD")
	for _, f := range change.files {
		refuseOnRedWrite(t, r.dir, f[0], f[1])
	}
	r.git(t, "add", ".")
	r.git(t, "commit", "-q", "-m", change.message)
	return r
}

func refuseOnRedRun(t *testing.T, r *refuseOnRedRepo, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{refuseOnRedAbs(t)}, args...)...)
	cmd.Dir = r.dir
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
		t.Fatalf("run %s: %v", refuseOnRedScript, err)
	}
	return out.String(), errb.String(), code
}

// refuseOnRedStub writes a kfuzz stand-in that prints the log named by -kernel and
// -surface. It returns the env assignment that points the script at it.
func refuseOnRedStub(t *testing.T, logs map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	keys := make([]string, 0, len(logs))
	for k := range logs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		refuseOnRedWrite(t, dir, filepath.Join("logs", k+".log"), logs[k])
	}
	stub := filepath.Join(dir, "kfuzz.sh")
	body := `#!/bin/sh
kernel= surface=
while [ "$#" -gt 0 ]; do
	case "$1" in
	-kernel) kernel=$2; shift 2 ;;
	-surface) surface=$2; shift 2 ;;
	*) shift ;;
	esac
done
cat "` + filepath.Join(dir, "logs") + `/$kernel-$surface.log"
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"REFUSE_ON_RED_KFUZZ=bash " + stub}
}

// refuseOnRedRecorder writes a REFUSE_LIST_RUN runner that records its last argument and
// writes one refusal file under dirac/support/refused/ in r. It returns the runner and
// the log of the command lines it saw.
func refuseOnRedRecorder(t *testing.T, r *refuseOnRedRepo) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ran := filepath.Join(dir, "ran.log")
	rec := filepath.Join(dir, "rec.sh")
	refused := filepath.Join(r.dir, "dirac", "support", "refused")
	body := "#!/bin/sh\nfor a; do last=$a; done\nprintf '%s\\n' \"$last\" >> \"" + ran + "\"\n" +
		"mkdir -p \"" + refused + "\"\nprintf 'refused: %s\\n' \"$last\" > \"" + filepath.Join(refused, "card.txt") + "\"\n"
	if err := os.WriteFile(rec, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return rec, ran
}

func refuseOnRedCleanLogs() map[string]string {
	logs := map[string]string{}
	for _, set := range []string{"fdn", "blb"} {
		for _, surface := range []string{"autopay", "full"} {
			logs[set+"-"+surface] = refuseOnRedCleanLog
		}
	}
	return logs
}

func refuseOnRedEnv(stub []string) []string {
	return append([]string{"REFUSE_ON_RED_CORPUS_CHECK=true", "REFUSE_ON_RED_PAIRS=2"}, stub...)
}

func refuseOnRedLast(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return lines[len(lines)-1]
}

func TestRefuseOnRed(t *testing.T) {
	t.Run("no-engine-change", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{"README.md", "docs\n"}},
			message: "docs: explain the rule\n\n" + refuseOnRedRule + "\n",
		})
		out, errs, code := refuseOnRedRun(t, r, []string{"REFUSE_ON_RED_CORPUS_CHECK=false"}, r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if !strings.HasPrefix(out, "refuse-on-red: n/a (") || !strings.Contains(out, "engine/") {
			t.Fatalf("stdout = %q, want n/a for the missing engine change", out)
		}
	})

	t.Run("touches-dirac", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}, {refuseOnRedDiracGo, "package dirac\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		out, errs, code := refuseOnRedRun(t, r, []string{"REFUSE_ON_RED_CORPUS_CHECK=false"}, r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if !strings.HasPrefix(out, "refuse-on-red: n/a (") || !strings.Contains(out, "dirac/") {
			t.Fatalf("stdout = %q, want n/a for the dirac change", out)
		}
	})

	t.Run("no-rule-line", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: refuseOnRedNoRuleMsg,
		})
		out, errs, code := refuseOnRedRun(t, r, []string{"REFUSE_ON_RED_CORPUS_CHECK=false"}, r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if !strings.HasPrefix(out, "refuse-on-red: n/a (") || !strings.Contains(out, "Rule: CR") {
			t.Fatalf("stdout = %q, want n/a for the missing rule line", out)
		}
	})

	t.Run("no-corpus", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		// kfuzz is `false`: reaching it would be an exit 1, so exit 0 proves nothing ran.
		out, errs, code := refuseOnRedRun(t, r, []string{
			"REFUSE_ON_RED_CORPUS_CHECK=false", "REFUSE_ON_RED_KFUZZ=false",
		}, r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if out != "refuse-on-red: n/a (no corpus)\n" {
			t.Fatalf("stdout = %q, want the no-corpus n/a", out)
		}
	})

	t.Run("lockstep-holds", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		out, errs, code := refuseOnRedRun(t, r, refuseOnRedEnv(refuseOnRedStub(t, refuseOnRedCleanLogs())), r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if got, want := refuseOnRedLast(out), "refuse-on-red: lockstep holds (2 pairs a slice)"; got != want {
			t.Fatalf("last stdout line = %q, want %q", got, want)
		}
	})

	t.Run("report-only-prints-commands-and-commits-nothing", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		logs := refuseOnRedCleanLogs()
		logs["fdn-autopay"] = refuseOnRedBadGame
		out, errs, code := refuseOnRedRun(t, r, refuseOnRedEnv(refuseOnRedStub(t, logs)), r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if !strings.Contains(out, "refuse-on-red: would refuse 1 cards (report-only):\n") {
			t.Fatalf("stdout lacks the report header:\n%s", out)
		}
		line := refuseOnRedLast(out)
		if !strings.HasPrefix(line, "go run ./dirac/cmd/kgen -refuse ") ||
			!strings.Contains(line, "gorge follows "+refuseOnRedRule+"; dirac not yet") ||
			!strings.Contains(line, "-repro fdn:autopay:7") {
			t.Fatalf("command line = %q", line)
		}
		if status := r.git(t, "status", "--porcelain"); status != "" {
			t.Fatalf("report-only changed the tree:\n%s", status)
		}
		if n := r.git(t, "rev-list", "--count", "HEAD"); n != "2" {
			t.Fatalf("report-only committed: %s commits, want 2", n)
		}
	})

	t.Run("apply-commits-refusals", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		logs := refuseOnRedCleanLogs()
		logs["fdn-autopay"] = refuseOnRedBadGame
		rec, ran := refuseOnRedRecorder(t, r)
		env := append(refuseOnRedEnv(refuseOnRedStub(t, logs)),
			"REFUSE_ON_RED=apply", "REFUSE_LIST_RUN="+rec)
		out, errs, code := refuseOnRedRun(t, r, env, r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if got, want := refuseOnRedLast(out), "refuse-on-red: refused 1 cards"; got != want {
			t.Fatalf("last stdout line = %q, want %q", got, want)
		}
		if status := r.git(t, "status", "--porcelain"); status != "" {
			t.Fatalf("apply left the tree dirty:\n%s", status)
		}
		if got, want := r.git(t, "log", "-1", "--format=%s"), "dirac: refuse 1 cards the rules fix diverges on"; got != want {
			t.Fatalf("commit subject = %q, want %q", got, want)
		}
		if body := r.git(t, "log", "-1", "--format=%b"); !strings.Contains(body, refuseOnRedRule) {
			t.Fatalf("commit body = %q, want the rule line", body)
		}
		if files := r.git(t, "ls-files", "dirac/support/refused"); files == "" {
			t.Fatal("the recorder's refusal was not staged and committed")
		}
		ranCommands, err := os.ReadFile(ran)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(ranCommands), `-refuse "Card A"`) {
			t.Fatalf("recorder saw:\n%s", ranCommands)
		}
	})

	t.Run("duplicate-card-counted-once", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		// One card, two surfaces: refuse-list names it in each slice, but it is one card.
		logs := refuseOnRedCleanLogs()
		logs["fdn-autopay"] = "pair 0 seed 1 ok\nbad-game\t7\t907\tdivergent\tlife\tCard A\t\tPlains\n"
		logs["fdn-full"] = "pair 0 seed 1 ok\nbad-game\t3\t903\tdivergent\tlife\tCard A\t\tPlains\n"
		out, errs, code := refuseOnRedRun(t, r, refuseOnRedEnv(refuseOnRedStub(t, logs)), r.base)
		if code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, errs)
		}
		if !strings.Contains(out, "refuse-on-red: would refuse 1 cards (report-only):\n") {
			t.Fatalf("stdout counts the card twice:\n%s", out)
		}
		if n := strings.Count(out, `-refuse "Card A"`); n != 1 {
			t.Fatalf("the card was printed %d times, want once:\n%s", n, out)
		}
	})

	t.Run("too-many-cards-exit-1", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the cards\n\n" + refuseOnRedRule + "\n",
		})
		var b strings.Builder
		for i := 0; i < 13; i++ {
			fmt.Fprintf(&b, "bad-game\t%d\t%d\tdivergent\tlife\tCard %02d\t\tPlains\n", i, i, i)
		}
		logs := refuseOnRedCleanLogs()
		logs["fdn-autopay"] = b.String()
		out, errs, code := refuseOnRedRun(t, r, refuseOnRedEnv(refuseOnRedStub(t, logs)), r.base)
		if code != 1 {
			t.Fatalf("exit %d, want 1; stdout:\n%s\nstderr:\n%s", code, out, errs)
		}
		if !strings.HasPrefix(out, "refuse-on-red: needs a person: ") || !strings.Contains(out, "more than") {
			t.Fatalf("stdout = %q, want needs-a-person for the card count", out)
		}
		if status := r.git(t, "status", "--porcelain"); status != "" {
			t.Fatalf("the failed run changed the tree:\n%s", status)
		}
	})

	t.Run("unnamed-game-exit-1", func(t *testing.T) {
		r := newRefuseOnRedRepo(t, refuseOnRedChange{
			files:   [][2]string{{refuseOnRedEngineGo, "package rules\n"}},
			message: "fix(rules): keep the card\n\n" + refuseOnRedRule + "\n",
		})
		logs := refuseOnRedCleanLogs()
		logs["blb-full"] = "pair 0 seed 1 ok\nbad-game\t9\t909\tdivergent\tmana\t\t\tForest\n"
		out, errs, code := refuseOnRedRun(t, r, refuseOnRedEnv(refuseOnRedStub(t, logs)), r.base)
		if code != 1 {
			t.Fatalf("exit %d, want 1; stdout:\n%s\nstderr:\n%s", code, out, errs)
		}
		if !strings.HasPrefix(out, "refuse-on-red: needs a person: ") || !strings.Contains(out, "no card named") {
			t.Fatalf("stdout = %q, want needs-a-person for the unnamed game", out)
		}
	})
}

func TestPrecheckCallsRefuseOnRed(t *testing.T) {
	b, err := os.ReadFile("pr-precheck.sh")
	if err != nil {
		t.Fatal(err)
	}
	const call = `bash scripts/refuse-on-red.sh "$base"`
	if got := strings.Count(string(b), call); got != 1 {
		t.Fatalf("pr-precheck.sh has the call %d times, want exactly once", got)
	}
	const block = `if [ -x scripts/refuse-on-red.sh ]; then ` + call + ` || fail "refuse-on-red: needs a person"; fi`
	body := string(b)
	at := strings.Index(body, block)
	if at < 0 {
		t.Fatalf("pr-precheck.sh lacks the refuse-on-red block:\n%s", block)
	}
	// The decision must run before the pass stamps: a needs-a-person exit 1 (or an
	// apply-mode refusal commit) must never be recorded as a passed head.
	const stamp = `git rev-parse HEAD >"$(git rev-parse --git-path pr-precheck-ok)"`
	stampAt := strings.Index(body, stamp)
	if stampAt < 0 {
		t.Fatalf("pr-precheck.sh lacks the git-dir pass stamp:\n%s", stamp)
	}
	if at > stampAt {
		t.Fatalf("the refuse-on-red block (offset %d) runs after the pass stamp (offset %d): a failure would be stamped as a pass", at, stampAt)
	}
}
