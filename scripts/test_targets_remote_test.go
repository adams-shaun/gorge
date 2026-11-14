package scripts

// Tests of the remote runner behind scripts/test-targets.sh (scripts/test-remote.sh, test-remote-box.sh): runner
// selection against a stub box (an ssh that runs the command on this machine, the box being a temp directory),
// the fallback when the box is unreachable, busy or fails to set up, and that only a test failure is FAIL.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const stubSSH = `#!/bin/bash
# A stub ssh: strip the options and the host, run the command here. STUB_SSH_DOWN: unreachable.
# STUB_SSH_DROP_RUN: the connection drops during the run.
[ -z "$STUB_SSH_DOWN" ] || { echo "ssh: connect to host box port 22: Connection timed out" >&2; exit 255; }
while [ $# -gt 0 ]; do
  case $1 in -i|-o|-F|-p|-l) shift 2 ;; -*) shift ;; *) break ;; esac
done
shift
case "$*" in *" -- run "*) [ -z "$STUB_SSH_DROP_RUN" ] || { echo "client_loop: send disconnect: Broken pipe" >&2; exit 255; } ;; esac
exec bash -c "$*"
`

const stubGo = `#!/bin/sh
case "$1" in
list) echo m ;;
env) echo /usr/local/go ;;
test)
  [ -z "$ECHO_FILE" ] || cp "$ECHO_FILE" "$ECHO_OUT" 2>/dev/null
  echo "$STREAM_MARK" >>"$MARKS"
  cat "$STREAM"
  exit "${GORC:-0}" ;;
esac
`

const passStream = `{"Action":"run","Package":"m/p","Test":"TestA"}
{"Action":"pass","Package":"m/p","Test":"TestA"}
{"Action":"pass","Package":"m/p"}
`

const failStream = `{"Action":"fail","Package":"m/p","Test":"TestA"}
{"Action":"fail","Package":"m/p"}
`

type rig struct {
	t    *testing.T
	repo string
	box  string
	bin  string
	env  []string
}

func sh(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	c := exec.Command(name, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func newRig(t *testing.T) *rig {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	r := &rig{t: t, repo: filepath.Join(root, "repo"), box: filepath.Join(root, "box"), bin: filepath.Join(root, "bin")}
	for _, d := range []string{r.repo, r.box, r.bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sh(t, r.repo, "git", "init", "-q", "-b", "land")
	write(t, filepath.Join(r.repo, "go.mod"), "module m\n\ngo 1.22\n", 0o644)
	write(t, filepath.Join(r.repo, ".gitignore"), ".cards\n.ds4\n", 0o644)
	write(t, filepath.Join(r.repo, "tracked.txt"), "committed\n", 0o644)
	for _, n := range []string{"test-targets.sh", "test_targets.py", "test-remote.sh", "test-remote-box.sh"} {
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(r.repo, "scripts", n), string(b), 0o755)
	}
	write(t, filepath.Join(r.repo, ".cards", "cardsfolder", "a.txt"), "x", 0o644)
	write(t, filepath.Join(r.repo, ".cards", "cards.lock"), "lock v1\n", 0o644)
	sh(t, r.repo, "git", "add", "-A")
	sh(t, r.repo, "git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x")
	sh(t, r.box, "git", "init", "-q", "--bare", "repo.git")
	write(t, filepath.Join(r.bin, "ssh"), stubSSH, 0o755)
	write(t, filepath.Join(r.bin, "go"), stubGo, 0o755)
	stream := filepath.Join(root, "stream.json")
	write(t, stream, passStream, 0o644)
	key := filepath.Join(root, "key")
	write(t, key, "not a key", 0o600)
	r.env = []string{
		"PATH=" + r.bin + ":" + os.Getenv("PATH"),
		"HOME=" + root,
		"TEST_TARGETS_SCOPED=1",
		"BASE=land",
		"GORGE_TEST_TARGETS_ENV=" + filepath.Join(root, "none.env"),
		"STREAM=" + stream,
		"MARKS=" + filepath.Join(root, "marks"),
		"PKG=./p", "RUN=TestA",
		"GORGE_REMOTE_HOST=ubuntu@box",
		"GORGE_REMOTE_KEY=" + key,
		"GORGE_REMOTE_REPO=" + filepath.Join(r.box, "repo.git"),
		"GORGE_REMOTE_MIN_IDLE=0",
		"GORGE_REMOTE_CARDS=" + filepath.Join(r.repo, ".cards"),
	}
	return r
}

func (r *rig) set(kv ...string) { r.env = append(r.env, kv...) }

// run is `bash scripts/test-targets.sh mode`: stdout, stderr and the exit code.
func (r *rig) run(mode string) (string, string, int) {
	r.t.Helper()
	c := exec.Command("bash", "scripts/test-targets.sh", mode)
	c.Dir = r.repo
	c.Env = r.env
	var o, e bytes.Buffer
	c.Stdout, c.Stderr = &o, &e
	err := c.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		r.t.Fatal(err)
	}
	return o.String(), e.String(), code
}

func lastLine(s string) string {
	l := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return l[len(l)-1]
}

// verdictLines are the lines of out that start with "test-": there must be exactly one, and it is the last.
func verdictLines(t *testing.T, out string) []string {
	t.Helper()
	var v []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "test-") {
			v = append(v, l)
		}
	}
	if len(v) != 1 || lastLine(out) != v[0] {
		t.Fatalf("want exactly one verdict line, and last; got %q in\n%s", v, out)
	}
	return v
}

// ranRemotely: the box ran it (go test ran once, in the box's worktree) and nothing ran here.
func (r *rig) goTestRuns() int {
	b, _ := os.ReadFile(filepath.Join(filepath.Dir(r.repo), "marks"))
	return strings.Count(string(b), "\n")
}

func (r *rig) boxIsClean() {
	r.t.Helper()
	if refs := sh(r.t, r.box, "git", "-C", "repo.git", "for-each-ref", "refs/tt"); strings.TrimSpace(refs) != "" {
		r.t.Errorf("the box kept refs after the run:\n%s", refs)
	}
	ents, _ := os.ReadDir(filepath.Join(r.box, "tt-wt"))
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), ".slot") {
			r.t.Errorf("the box kept %s after the run", e.Name())
		}
	}
	if wl := sh(r.t, r.box, "git", "-C", "repo.git", "worktree", "list", "--porcelain"); strings.Count(wl, "worktree ") != 1 {
		r.t.Errorf("the box kept a scratch worktree:\n%s", wl)
	}
}

func TestTestTargetsRemoteReachableRunsOnTheBox(t *testing.T) {
	r := newRig(t)
	out, errs, code := r.run("one")
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errs)
	}
	v := verdictLines(t, out)[0]
	if !strings.HasPrefix(v, "test-one: PASS ") || !strings.Contains(v, " ./p ") {
		t.Errorf("verdict %q", v)
	}
	if !strings.Contains(out, "== remote: ubuntu@box") {
		t.Errorf("the run did not go to the box:\n%s", out)
	}
	if strings.Contains(errs, "running locally") {
		t.Errorf("fell back: %s", errs)
	}
	if n := r.goTestRuns(); n != 1 {
		t.Errorf("go test ran %d times, want 1 (on the box)", n)
	}
	// The full go test -json is at the usual local path.
	sha := strings.Fields(v)[2]
	b, err := os.ReadFile(filepath.Join(r.repo, ".ds4", "scratch", "test", "one-"+sha+".json"))
	if err != nil || !strings.Contains(string(b), `"Test":"TestA"`) {
		t.Errorf("the json stream did not come back: %v %q", err, b)
	}
	r.boxIsClean()
}

func TestTestTargetsRemoteSameVerdictAsLocal(t *testing.T) {
	r := newRig(t)
	remote, _, _ := r.run("one")
	r.set("GORGE_REMOTE_OFF=1")
	local, errs, code := r.run("one")
	if code != 0 || strings.Contains(local, "== remote") || strings.Contains(errs, "remote") {
		t.Fatalf("the off switch did not keep it local: %d\n%s\n%s", code, local, errs)
	}
	rf, lf := strings.Fields(verdictLines(t, remote)[0]), strings.Fields(verdictLines(t, local)[0])
	// test-one: STATUS sha pkgs seconds: everything but the seconds is the same.
	if strings.Join(rf[:4], " ") != strings.Join(lf[:4], " ") {
		t.Errorf("remote %q, local %q", rf, lf)
	}
}

func TestTestTargetsRemoteUnsetIsLocalOnly(t *testing.T) {
	r := newRig(t)
	var kept []string
	for _, kv := range r.env {
		if !strings.HasPrefix(kv, "GORGE_REMOTE_HOST=") {
			kept = append(kept, kv)
		}
	}
	r.env = kept
	r.set("STUB_SSH_DOWN=1") // would fail loudly if anything tried the box
	out, errs, code := r.run("one")
	if code != 0 || strings.Contains(out, "== remote") || strings.Contains(errs, "remote") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	verdictLines(t, out)
}

func TestTestTargetsRemoteUnreachableFallsBackLocally(t *testing.T) {
	r := newRig(t)
	r.set("STUB_SSH_DOWN=1")
	out, errs, code := r.run("one")
	if code != 0 {
		t.Fatalf("an unreachable box must not fail the run: exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Errorf("verdict:\n%s", out)
	}
	if !strings.Contains(errs, "running locally") || !strings.Contains(errs, "unreachable") {
		t.Errorf("no fallback note:\n%s", errs)
	}
	if r.goTestRuns() != 1 {
		t.Errorf("go test ran %d times, want 1 (here)", r.goTestRuns())
	}
}

func TestTestTargetsRemoteBusyFallsBackLocally(t *testing.T) {
	cases := map[string]func(r *rig){
		"too few idle cores": func(r *rig) { r.set("GORGE_REMOTE_MIN_IDLE=1000000") },
		"no run slot":        func(r *rig) { r.set("GORGE_REMOTE_MAX_RUNS=0") },
		"slot held by another run": func(r *rig) {
			r.set("GORGE_REMOTE_MAX_RUNS=1")
			if err := os.MkdirAll(filepath.Join(r.box, "tt-wt"), 0o755); err != nil {
				t.Fatal(err)
			}
			f, err := os.OpenFile(filepath.Join(r.box, "tt-wt", ".slot1"), os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { f.Close() })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			setup(r)
			out, errs, code := r.run("one")
			if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
				t.Fatalf("busy box: exit %d\n%s\n%s", code, out, errs)
			}
			if !strings.Contains(errs, "busy") || !strings.Contains(errs, "running locally") || strings.Contains(out, "== remote") {
				t.Errorf("expected a busy fallback note:\n%s", errs)
			}
			if r.goTestRuns() != 1 {
				t.Errorf("go test ran %d times, want 1 (here)", r.goTestRuns())
			}
		})
	}
}

// An infrastructure failure is never a FAIL: the run goes local, which here passes.
func TestTestTargetsRemoteInfrastructureFailureFallsBackNotFail(t *testing.T) {
	cases := map[string]func(r *rig){
		"the connection drops mid-run": func(r *rig) { r.set("STUB_SSH_DROP_RUN=1") },
		"the box repository is missing": func(r *rig) {
			r.set("GORGE_REMOTE_REPO=" + filepath.Join(r.box, "nope", "repo.git"))
		},
		"the box has a different corpus": func(r *rig) {
			write(r.t, filepath.Join(r.repo, ".cards", "cards.lock"), "lock v2\n", 0o644)
			other := filepath.Join(filepath.Dir(r.repo), "othercards")
			write(r.t, filepath.Join(other, "cardsfolder", "a.txt"), "x", 0o644)
			write(r.t, filepath.Join(other, "cards.lock"), "lock v1\n", 0o644)
			r.set("GORGE_REMOTE_CARDS=" + other)
		},
		"the box has no corpus": func(r *rig) {
			r.set("GORGE_REMOTE_CARDS=" + filepath.Join(filepath.Dir(r.repo), "missing"))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			setup(r)
			out, errs, code := r.run("one")
			v := verdictLines(t, out)[0]
			if code != 0 || !strings.HasPrefix(v, "test-one: PASS ") || strings.Contains(v, "FAIL") {
				t.Fatalf("an infrastructure failure must fall back, not fail: exit %d\n%s\n%s", code, out, errs)
			}
			if !strings.Contains(errs, "running locally") {
				t.Errorf("no fallback note:\n%s", errs)
			}
			r.boxIsClean()
		})
	}
}

func TestTestTargetsRemoteTestFailureIsFail(t *testing.T) {
	r := newRig(t)
	write(t, filepath.Join(filepath.Dir(r.repo), "stream.json"), failStream, 0o644)
	r.set("GORC=1")
	out, errs, code := r.run("one")
	if code != 1 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: FAIL ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(out, "== remote") || strings.Contains(errs, "running locally") || r.goTestRuns() != 1 {
		t.Errorf("a failing test on the box is the answer; it must not rerun here:\n%s\n%s (go test ran %d times)", out, errs, r.goTestRuns())
	}
	r.boxIsClean()
}

// A BUDGET on the box stands: it has more cores than a run here, which would only run out of time again.
func TestTestTargetsRemoteBoxBudgetStands(t *testing.T) {
	r := newRig(t)
	write(t, filepath.Join(r.bin, "go"), `#!/bin/sh
case "$1" in
list) echo m ;;
env) echo /usr/local/go ;;
test)
  echo x >>"$MARKS"
  printf '%s\n' '{"Action":"output","Package":"m/p","Output":"panic: test timed out after 2m0s"}' '{"Action":"fail","Package":"m/p"}'; exit 2 ;;
esac
`, 0o755)
	out, errs, code := r.run("one")
	if code != 5 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: BUDGET ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if strings.Contains(errs, "running locally") || r.goTestRuns() != 1 {
		t.Errorf("the box's BUDGET must stand, not rerun here (go test ran %d times):\n%s", r.goTestRuns(), errs)
	}
	r.boxIsClean()
}

func TestTestTargetsRemoteShipsTheUncommittedDiff(t *testing.T) {
	r := newRig(t)
	write(t, filepath.Join(r.repo, "tracked.txt"), "edited\n", 0o644)
	write(t, filepath.Join(r.repo, "fresh.txt"), "untracked\n", 0o644)
	r.set("ECHO_FILE=tracked.txt", "ECHO_OUT="+filepath.Join(filepath.Dir(r.repo), "tracked.seen"))
	out, errs, code := r.run("one")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	v := verdictLines(t, out)[0]
	if !strings.Contains(v, "+dirty") || !strings.Contains(v, " ./p ") {
		t.Errorf("verdict %q", v)
	}
	if b, _ := os.ReadFile(filepath.Join(filepath.Dir(r.repo), "tracked.seen")); string(b) != "edited\n" {
		t.Errorf("the box did not see the uncommitted edit: %q", b)
	}
	// The local worktree is untouched (the patch is made with a private index).
	if st := sh(t, r.repo, "git", "status", "--porcelain"); strings.Contains(st, "A ") {
		t.Errorf("the local index changed:\n%s", st)
	}
	r.boxIsClean()
}

func TestTestTargetsRemoteLocalOnlyRuns(t *testing.T) {
	// A run that writes files (UPDATE, TESTENV) and test-smart on a dirty tree are never shipped.
	for name, kv := range map[string]string{"update": "UPDATE=1", "testenv": "TESTENV=KGEN_REPIN=1"} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			r.set(kv, "STUB_SSH_DOWN=1")
			out, errs, code := r.run("one")
			if code != 0 || strings.Contains(errs, "remote") || strings.Contains(out, "== remote") {
				t.Fatalf("exit %d\n%s\n%s", code, out, errs)
			}
		})
	}
	r := newRig(t)
	write(t, filepath.Join(r.repo, "tracked.txt"), "dirty\n", 0o644)
	r.set("STUB_SSH_DOWN=1")
	out, errs, code := r.run("smart")
	if code != 3 || !strings.HasPrefix(verdictLines(t, out)[0], "test-smart: ENV ") || strings.Contains(errs, "remote") {
		t.Fatalf("smart on a dirty tree: exit %d\n%s\n%s", code, out, errs)
	}
}

func TestTestTargetsRemoteSettingsFromTheEnvFile(t *testing.T) {
	r := newRig(t)
	var kept, file []string
	for _, kv := range r.env {
		if strings.HasPrefix(kv, "GORGE_REMOTE_") {
			file = append(file, kv)
		} else if !strings.HasPrefix(kv, "GORGE_TEST_TARGETS_ENV=") {
			kept = append(kept, kv)
		}
	}
	envfile := filepath.Join(filepath.Dir(r.repo), "test-targets.env")
	write(t, envfile, strings.Join(file, "\n")+"\n", 0o644)
	r.env = append(kept, "GORGE_TEST_TARGETS_ENV="+envfile)
	out, errs, code := r.run("one")
	if code != 0 || !strings.Contains(out, "== remote: ubuntu@box") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
}
