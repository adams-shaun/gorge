package gorgedevents_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// eventsServerDir is the persistence directory for an events test's server:
// a RAM-backed directory when the platform offers one, t.TempDir() otherwise.
//
// The served server fsyncs every event-log line (draft/eventhost store's
// appendLog) and every match-journal burst (server/host persist's append
// writes the intent and the burst's events with a Sync each). One all-bot
// Bo3 pod writes ~16,000 of them: 14 games of ~600 decision bursts, each
// burst two fsyncs. On this machine's idle NVMe those fsyncs cost ~20µs
// each and the Bo3 pod test runs in 8s on ext4 with one CPU (0.99s on
// tmpfs); on a loaded CI runner, where one fsync costs milliseconds, the
// same test passed 50s (promote PR #184, job test (4)). No assertion in
// these tests reads media durability — the log file is still created,
// appended and asserted under <dir>/events/e1, and the fsync calls
// themselves still run — only the storage's latency changes, so the event
// lifecycle tests run their persistence on RAM when they can.
func eventsServerDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		return t.TempDir()
	}
	dir, err := os.MkdirTemp("/dev/shm", "gorgedevents-")
	if err != nil {
		return t.TempDir()
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// TestEventsServerDir pins the helper's contract: the directory holds the
// server's writes (create, append, read back), and when /dev/shm is
// writable the server dir is RAM-backed rather than the disk-backed temp
// dir — the whole point of the helper. A /dev/shm that cannot host the dir
// (a container without one) falls back to t.TempDir() and stays disk-backed.
func TestEventsServerDir(t *testing.T) {
	dir := eventsServerDir(t)
	f := filepath.Join(dir, "probe.jsonl")
	if err := os.WriteFile(f, []byte("{\"kind\":\"EventCreated\"}\n"), 0o644); err != nil {
		t.Fatalf("write in server dir: %v", err)
	}
	b, err := os.ReadFile(f)
	if err != nil || string(b) != "{\"kind\":\"EventCreated\"}\n" {
		t.Fatalf("read back %q (%v), want the written line", b, err)
	}

	// The RAM-backed branch, asserted only where its precondition holds:
	// a writable /dev/shm. When the helper had no such branch, or MkdirTemp
	// lost the error, this fails on every machine that has /dev/shm.
	ram, ramErr := os.MkdirTemp("/dev/shm", "gorgedevents-probe-")
	switch {
	case ramErr == nil:
		os.RemoveAll(ram)
		if !strings.HasPrefix(dir, "/dev/shm"+string(filepath.Separator)) {
			t.Fatalf("server dir %q is not RAM-backed though /dev/shm is writable", dir)
		}
	case runtime.GOOS == "linux":
		if strings.HasPrefix(dir, "/dev/shm"+string(filepath.Separator)) {
			t.Fatalf("server dir %q is RAM-backed though /dev/shm refused a directory", dir)
		}
	}
}
