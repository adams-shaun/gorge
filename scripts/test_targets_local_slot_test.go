package scripts

// The host-wide local run slot (scripts/test-targets.sh, GORGE_LOCAL_SLOTS): a local run waits for one of the
// slots and gives BUSY when none frees up in GORGE_LOCAL_SLOT_WAIT seconds (11-23: 22 seat runs at once, load 99).

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// slotRig is a rig whose runs take the unscoped path (where the slot is taken) with no user systemd.
func slotRig(t *testing.T, slots string) (*rig, string) {
	t.Helper()
	r := newRig(t)
	dir := filepath.Join(filepath.Dir(r.repo), "slots")
	write(t, filepath.Join(r.bin, "systemd-run"), "#!/bin/sh\nexit 1\n", 0o755)
	r.set("TEST_TARGETS_SCOPED=", "GORGE_REMOTE_MODES=targeted smart", "STUB_SSH_DOWN=1",
		"GORGE_LOCAL_SLOTS="+slots, "GORGE_LOCAL_SLOT_DIR="+dir, "GORGE_LOCAL_SLOT_WAIT=1")
	return r, dir
}

func TestTestTargetsLocalSlotBusyWhenAllTaken(t *testing.T) {
	t.Parallel()
	r, dir := slotRig(t, "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "slot1"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	out, errs, code := r.run("one")
	if code != 75 || !strings.HasPrefix(lastLine(out), "test-one: BUSY") {
		t.Fatalf("held slot: want BUSY (75), got %d\nstdout:\n%s\nstderr:\n%s", code, out, errs)
	}
}

func TestTestTargetsLocalSlotRunsWhenFree(t *testing.T) {
	t.Parallel()
	r, _ := slotRig(t, "2")
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(lastLine(out), "test-one: PASS") {
		t.Fatalf("free slot: want PASS, got %d\nstdout:\n%s\nstderr:\n%s", code, out, errs)
	}
}

func TestTestTargetsLocalSlotOffAtZero(t *testing.T) {
	t.Parallel()
	r, dir := slotRig(t, "0")
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(lastLine(out), "test-one: PASS") {
		t.Fatalf("slots off: want PASS, got %d\nstdout:\n%s\nstderr:\n%s", code, out, errs)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("slots off: %s was created", dir)
	}
}
