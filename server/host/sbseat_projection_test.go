package host

import (
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
)

// projectionRecordingSeat wraps a SpellBench v2 Agent and records the match
// context the host attaches, so the test can assert the host supplied the
// projection lock the agent's Derived reads need.
type projectionRecordingSeat struct {
	seat.Seat
	mu sync.Mutex
	mc *bots.MatchContext
}

func (s *projectionRecordingSeat) AttachMatch(mc *bots.MatchContext) error {
	s.mu.Lock()
	s.mc = mc
	s.mu.Unlock()
	return bots.AttachMatch(s.Seat, mc)
}

func (s *projectionRecordingSeat) EndMatch(result bots.MatchResult) {
	bots.EndMatch(s.Seat, result)
}

var _ bots.MatchSeat = (*projectionRecordingSeat)(nil)
var _ bots.MatchEnd = (*projectionRecordingSeat)(nil)

// TestHostProjectsAgentSeatUnderMatchLock seats a real v2 Agent against the
// production bot with a focus subscriber draining the whole match. The host
// must hand the agent a Projection hook (the same exclusive m.mu projectLive
// takes), and the match must finish cleanly while that fanout runs.
func TestHostProjectsAgentSeatUnderMatchLock(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	script, err := filepath.Abs(filepath.Join("..", "spellbench", "seat", "testdata", "minimal_bot.py"))
	if err != nil {
		t.Fatal(err)
	}
	const inner, probe = "host-test-sb-inner", "host-test-sb-projection"
	if err := sbseat.Register(inner, []string{python, script}); err != nil {
		t.Fatal(err)
	}
	base, ok := bots.Lookup(inner)
	if !ok {
		t.Fatal("registered SpellBench policy is missing")
	}
	info := base.Info
	info.Name = probe
	var built []*projectionRecordingSeat
	var builtMu sync.Mutex
	bots.Register(bots.Entry{Info: info, New: func(o bots.Options) (seat.Seat, error) {
		inner, err := base.New(o)
		if err != nil {
			return nil, err
		}
		s := &projectionRecordingSeat{Seat: inner}
		builtMu.Lock()
		built = append(built, s)
		builtMu.Unlock()
		return s, nil
	}})

	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if err := r.AddTable(TableConfig{
		ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 11, Pace: 0,
		SeatBotPolicies: []string{probe, bots.Default}, Spectator: view.Omniscient,
		AutoMana: true, BotAutoPayMana: true,
	}); err != nil {
		t.Fatal(err)
	}
	// Subscribe before Start: the match's own fanout then projects live views
	// for this session on every burst, concurrently with the agent's Decides.
	s := r.OpenSession()
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range s.Out() {
		}
	}()
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	<-drained

	builtMu.Lock()
	seats := append([]*projectionRecordingSeat(nil), built...)
	builtMu.Unlock()
	if len(seats) != 1 {
		t.Fatalf("built %d SpellBench seats, want 1", len(seats))
	}
	seats[0].mu.Lock()
	mc := seats[0].mc
	seats[0].mu.Unlock()
	if mc == nil {
		t.Fatal("SpellBench seat was not attached")
	}
	if mc.Projection == nil {
		t.Fatal("host attached a SpellBench seat without a Projection lock")
	}
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches = %v, %v", ms, err)
	}
	if ms[0].State != protocol.MatchFinished {
		t.Fatalf("match state = %q, want finished", ms[0].State)
	}
	if ms[0].Head == "" || ms[0].Events == 0 {
		t.Fatalf("match has no recorded events: %+v", ms[0])
	}
}
