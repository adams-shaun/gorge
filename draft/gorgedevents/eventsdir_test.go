package gorgedevents_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eventsServerDir is the persistence directory for an events test's server:
// the test's own disk-backed t.TempDir(), so the event lifecycle tests
// exercise the real durability path. A match journal defers its per-burst
// fsyncs when the match is all-bot (server/host matchJournalSync), and the
// event log still fsyncs each appended line (draft/eventhost store's
// appendLog). One all-bot Bo3 pod therefore writes on the order of 300
// fsyncs, not the ~16,000 it wrote while every all-bot journal burst was
// synced twice, so the pod E2E stays well under its 50s deadline on a loaded
// CI disk. No assertion in these tests reads media durability: the log file
// is still created, appended and asserted under <dir>/events/e1. Do not point
// this at a RAM-backed directory; that would hide the production path this
// helper exists to exercise.
func eventsServerDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// TestEventsServerDir pins the helper's contract: the directory is the test's
// disk-backed temp dir (under the process temp dir) and holds the server's
// writes (create, append, read back).
func TestEventsServerDir(t *testing.T) {
	dir := eventsServerDir(t)
	rel, err := filepath.Rel(os.TempDir(), dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("server dir %q is not under the process temp dir %q", dir, os.TempDir())
	}
	f := filepath.Join(dir, "probe.jsonl")
	if err := os.WriteFile(f, []byte("{\"kind\":\"EventCreated\"}\n"), 0o644); err != nil {
		t.Fatalf("write in server dir: %v", err)
	}
	b, err := os.ReadFile(f)
	if err != nil || string(b) != "{\"kind\":\"EventCreated\"}\n" {
		t.Fatalf("read back %q (%v), want the written line", b, err)
	}
}
