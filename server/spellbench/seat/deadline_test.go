package seat

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// firedDeadline is a context the test fires explicitly once the helper agent
// has consumed the request. It reports DeadlineExceeded so the seat
// classifies the forfeit as a timeout without depending on a wall-clock
// window.
type firedDeadline struct {
	done chan struct{}
	once sync.Once
}

func newFiredDeadline() *firedDeadline {
	return &firedDeadline{done: make(chan struct{})}
}

func (c *firedDeadline) fire() {
	c.once.Do(func() { close(c.done) })
}

func (c *firedDeadline) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *firedDeadline) Done() <-chan struct{}       { return c.done }
func (c *firedDeadline) Value(any) any               { return nil }

func (c *firedDeadline) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

// waitMarker polls for the helper agent's marker file with a generous ceiling.
// It is safe to call from a goroutine: it reports failure instead of calling
// t.Fatal.
func waitMarker(path string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(time.Second / 100)
	}
}

func awaitMarker(t *testing.T, path string) {
	t.Helper()
	if !waitMarker(path, 10*time.Second) {
		t.Fatalf("helper agent did not signal %s", path)
	}
}
