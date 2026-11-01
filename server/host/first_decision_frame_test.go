package host

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
)

// With every seat human no Submit can run, so the only frame that can carry the
// first decision is the one the match publishes when it parks that decision.
func TestFirstDecisionReachesAnEarlySubscriber(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := fourSeatTable("t1", false)
	cfg.Humans = []int{0, 1, 2, 3}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	s := r.OpenSession()
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(30 * time.Second)
	for {
		select {
		case f := <-s.Out():
			if f.T == protocol.TDecision {
				return
			}
		case <-deadline:
			t.Fatal("the first decision never reached a subscriber waiting since before Start")
		}
	}
}
