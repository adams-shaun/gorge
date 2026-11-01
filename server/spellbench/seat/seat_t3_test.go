package seat

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestStartupBudgetKillsSilentHello(t *testing.T) {
	python(t)
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("SPELLBENCH_MARKER", marker)
	e, _, _ := testEngine(t)
	raw, err := New("test-agent", []string{script(t, "silent_hello_bot.py")}, bots.Options{})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	deadline := newFiredDeadline()
	agent.startup = func() (context.Context, context.CancelFunc) {
		return deadline, func() {}
	}
	mc := &bots.MatchContext{
		Context:      context.Background(),
		Game:         gorgegame.Game{E: e},
		GameID:       "g-0000000000000002",
		Seat:         0,
		Seats:        2,
		Format:       "constructed",
		Seed:         []byte("startup"),
		OwnDeck:      e.OwnDeck(state.PlayerID(0)),
		Mulligan:     "none",
		StartingSeat: int(e.G.Active),
		Logf:         t.Logf,
	}
	done := make(chan error, 1)
	go func() { done <- agent.AttachMatch(mc) }()
	awaitMarker(t, marker)
	deadline.fire()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("AttachMatch: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("startup budget did not interrupt a silent hello")
	}
	if agent.forfeit != forfeitTimeout {
		t.Fatalf("startup forfeit = %q, want %q", agent.forfeit, forfeitTimeout)
	}
}

func TestSeatSerializesDerivedReadsUnderProjection(t *testing.T) {
	e, _, _ := testEngine(t)
	argv := append(goRun(t), "./testdata/gorgeagent")
	fallback := heuristic.NewBot(1)
	raw, err := New("test-agent", argv, bots.Options{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	var mu sync.Mutex
	calls := 0
	mc := &bots.MatchContext{
		Context:      context.Background(),
		Game:         gorgegame.Game{E: e},
		Projection:   func(f func()) { mu.Lock(); calls++; mu.Unlock(); f() },
		GameID:       "g-0000000000000003",
		Seat:         0,
		Seats:        2,
		Format:       "constructed",
		Seed:         []byte("projection"),
		OwnDeck:      e.OwnDeck(state.PlayerID(0)),
		Mulligan:     "none",
		StartingSeat: int(e.G.Active),
		Logf:         t.Logf,
	}
	if err := agent.AttachMatch(mc); err != nil {
		t.Fatalf("AttachMatch: %v", err)
	}
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, v := nextAgentAsk(t, e, agent, fallback)
	mu.Lock()
	before := calls
	mu.Unlock()
	in, err := agent.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatal(err)
	}
	if in.Seq == 0 && d.Seq != 0 {
		t.Fatalf("Decide returned an unsequenced intent for decision %d", d.Seq)
	}
	mu.Lock()
	after := calls
	mu.Unlock()
	if after == before {
		t.Fatal("Decide read the engine without the host projection lock")
	}
}

func TestRoundTripWrapsWriteErrorsAsTransport(t *testing.T) {
	python(t)
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "minimal_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})
	if agent.cmd == nil || agent.cmd.Process == nil {
		t.Fatal("agent process is not running")
	}
	if err := agent.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = agent.cmd.Wait()
	_, err := agent.roundTrip(context.Background(), helloReq{RequestType: "hello", RequestID: "r-x"})
	if !errors.Is(err, errTransport) {
		t.Fatalf("write error = %v, want a wrapped errTransport", err)
	}
}
