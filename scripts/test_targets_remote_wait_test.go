package scripts

// The box-first settings of the remote runner (scripts/test-remote.sh): GORGE_REMOTE_MODES keeps the light modes
// here, GORGE_REMOTE_WAIT waits for a slot on a busy box instead of running here, a red while the landing gate
// shared the box is rerun there once, and a run that falls back is marked (TEST_TARGETS_FALLBACK) for the lower cap.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// holdSlot takes the box's run slot 1 and lets it go after d (0: at cleanup).
func holdSlot(t *testing.T, r *rig, d time.Duration) {
	t.Helper()
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
	if d > 0 {
		time.AfterFunc(d, func() { f.Close() })
	}
	t.Cleanup(func() { f.Close() })
}

// fallbackStubGo is the stub go that also records TEST_TARGETS_FALLBACK, where the run went.
func fallbackStubGo(t *testing.T, r *rig) string {
	t.Helper()
	seen := filepath.Join(filepath.Dir(r.repo), "fallback.seen")
	write(t, filepath.Join(r.bin, "go"), `#!/bin/sh
case "$1" in
list) echo m ;;
env) echo /usr/local/go ;;
test)
  echo "fallback=$TEST_TARGETS_FALLBACK remote=$TEST_TARGETS_REMOTE" >>"$SEEN"
  echo x >>"$MARKS"
  cat "$STREAM" ;;
esac
`, 0o755)
	r.set("SEEN=" + seen)
	return seen
}

func TestTestTargetsRemoteModesKeepTestOneHere(t *testing.T) {
	r := newRig(t)
	seen := fallbackStubGo(t, r)
	r.set("GORGE_REMOTE_MODES=targeted smart", "STUB_SSH_DOWN=1")
	out, errs, code := r.run("one")
	if code != 0 || strings.Contains(out, "== remote") || strings.Contains(errs, "remote") {
		t.Fatalf("test-one must stay here without trying the box: exit %d\n%s\n%s", code, out, errs)
	}
	if b, _ := os.ReadFile(seen); string(b) != "fallback= remote=\n" {
		t.Errorf("a run that never tried the box is not a fallback: %q", b)
	}
}

func TestTestTargetsRemoteWaitsForASlot(t *testing.T) {
	r := newRig(t)
	r.set("GORGE_REMOTE_MAX_RUNS=1", "GORGE_REMOTE_WAIT=60", "GORGE_REMOTE_POLL=1")
	holdSlot(t, r, 2*time.Second)
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(errs, "waiting for a slot") || !strings.Contains(out, "== remote") || strings.Contains(errs, "running locally") {
		t.Errorf("expected a wait, then the box:\n%s\n%s", out, errs)
	}
	r.boxIsClean()
}

func TestTestTargetsRemoteWaitRunsOutThenFallsBackCapped(t *testing.T) {
	r := newRig(t)
	seen := fallbackStubGo(t, r)
	r.set("GORGE_REMOTE_MAX_RUNS=1", "GORGE_REMOTE_WAIT=2", "GORGE_REMOTE_POLL=1")
	holdSlot(t, r, 0)
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(errs, "waiting for a slot") || !strings.Contains(errs, "running locally") || strings.Contains(out, "== remote") {
		t.Errorf("expected the wait to run out, then a run here:\n%s\n%s", out, errs)
	}
	if b, _ := os.ReadFile(seen); string(b) != "fallback=1 remote=\n" {
		t.Errorf("the run here must carry the fallback cap marker: %q", b)
	}
}

func TestTestTargetsRemoteUnreachableIsMarkedFallback(t *testing.T) {
	r := newRig(t)
	seen := fallbackStubGo(t, r)
	r.set("STUB_SSH_DOWN=1")
	out, errs, code := r.run("one")
	if code != 0 || !strings.Contains(errs, "running locally") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if b, _ := os.ReadFile(seen); string(b) != "fallback=1 remote=\n" {
		t.Errorf("an unreachable box falls back at the fallback cap: %q", b)
	}
}

// A red while the landing gate shared the box runs on the box once more; nothing runs here.
func TestTestTargetsRemoteContendedRedRerunsOnTheBox(t *testing.T) {
	r := newRig(t)
	// The landing gate's scope is active until the first go test has run.
	write(t, filepath.Join(r.bin, "systemctl"), `#!/bin/sh
case "$*" in *is-active*)
  if [ "$(cat "$MARKS" 2>/dev/null | wc -l)" -lt 1 ]; then echo active; exit 0; fi
  echo inactive; exit 3 ;;
esac
exit 0
`, 0o755)
	write(t, filepath.Join(r.bin, "go"), `#!/bin/sh
case "$1" in
list) echo m ;;
env) echo /usr/local/go ;;
test)
  n=$(cat "$MARKS" 2>/dev/null | wc -l)
  echo x >>"$MARKS"
  if [ "$n" = 0 ]; then printf '%s\n' '{"Action":"fail","Package":"m/p","Test":"TestA"}' '{"Action":"fail","Package":"m/p"}'; exit 1; fi
  cat "$STREAM" ;;
esac
`, 0o755)
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(errs, "running it on the box again") || strings.Contains(errs, "running locally") || r.goTestRuns() != 2 {
		t.Errorf("expected one rerun on the box (go test ran %d times):\n%s", r.goTestRuns(), errs)
	}
	r.boxIsClean()
}
