package extern

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
)

// TestPythonExampleAgent drives examples/extagent/random_agent.py -- the
// dependency-free Python twin of refagent -- through the wire as the
// published example: it handshakes as ext:py-random@1, plays one game
// against bot@1, and drafts/builds/plays a 4-seat FDN event (seats 0 and 2)
// whose every game tape re-drives with no agent at all. It skips honestly
// when python3 is not on PATH.
func TestPythonExampleAgent(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not on PATH")
	}
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", "extagent", "random_agent.py"))
	if err != nil {
		t.Fatalf("resolve example path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("example agent missing: %v", err)
	}

	// The script carries its own shebang and mode 755, so ext:exec: runs it
	// directly (operator decision 2026-10-14 #4: ext:exec: takes no argv).
	a, err := Dial(context.Background(), Endpoint{Transport: "exec", Target: path}, Options{Stderr: io.Discard})
	if err != nil {
		t.Fatalf("dial example agent: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	if got := a.Ref(); got != (draftlab.AgentRef{Name: "ext:py-random", Version: "1"}) {
		t.Fatalf("Ref = %s, want ext:py-random@1", got)
	}

	l := testLabOpts(t, draftlab.Options{MaxTurns: 40, RecordTape: true})
	bot := testBot(t, l)

	// One game against the built-in bot: the agent really answers a seat.
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	res, err := l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 4242)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if a.Stats().Decides == 0 {
		t.Fatal("the example agent was never asked a decide (precondition)")
	}
	t.Logf("game vs bot: winner=%d stall=%q turns=%d decides=%d",
		res.Winner, res.Stall, res.Turns, a.Stats().Decides)

	// A 4-seat event: the agent drafts and builds every seat and plays
	// seats 0 and 2; the tapes then re-drive with no agent.
	d, err := a.Drafter(l)
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	bld, err := a.Builder(l)
	if err != nil {
		t.Fatalf("Builder: %v", err)
	}
	p2, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	ev, err := l.RunEvent(draftlab.EventSpec{
		Set:       "FDN",
		Seats:     4,
		Structure: draftlab.RoundRobin,
		Drafters:  []draftlab.NamedDrafter{d},
		Builders:  []draftlab.NamedBuilder{bld},
		Players:   []draftlab.NamedPlayer{p2, bot, p2, bot},
		Seed:      77,
	})
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	st := a.Stats()
	if st.Picks == 0 || st.Builds == 0 || st.Decides == 0 {
		t.Fatalf("the example agent was not asked all three kinds: picks %d builds %d decides %d",
			st.Picks, st.Builds, st.Decides)
	}
	if len(ev.Games) == 0 {
		t.Fatal("the event played no games (precondition)")
	}
	if err := l.VerifyEventTapes(ev); err != nil {
		t.Fatalf("VerifyEventTapes: %v", err)
	}
	t.Logf("event: %d games, picks=%d builds=%d decides=%d",
		len(ev.Games), st.Picks, st.Builds, st.Decides)
}
