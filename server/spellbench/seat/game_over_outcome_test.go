package seat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

type discardCloser struct{ io.Writer }

func (discardCloser) Close() error { return nil }

func gameOverTerminal(t *testing.T, a *Agent, r bots.MatchResult) terminalNotice {
	t.Helper()
	var sent bytes.Buffer
	a.ctx = context.Background()
	a.stdin = discardCloser{&sent}
	a.reader = wire.NewReader(strings.NewReader(""))
	a.EndMatch(r)
	var req gameOverReq
	if err := json.Unmarshal(sent.Bytes(), &req); err != nil {
		t.Fatalf("game_over %q: %v", sent.String(), err)
	}
	return req.Terminal
}

func TestGameOverNeverSendsAnEmptyOutcome(t *testing.T) {
	halted := gameOverTerminal(t, &Agent{mc: &bots.MatchContext{Seat: 0}},
		bots.MatchResult{Classification: "halted", Reason: "not_finished"})
	if halted.Outcome != "halted" {
		t.Errorf("halted outcome = %q, want \"halted\"", halted.Outcome)
	}
	forfeit := gameOverTerminal(t, &Agent{mc: &bots.MatchContext{Seat: 0}, forfeit: forfeitTransport},
		bots.MatchResult{Classification: "natural", Outcome: "p0_win", Winner: "p0"})
	if forfeit.Classification != "forfeit" || forfeit.Winner == nil || *forfeit.Winner != "p1" {
		t.Fatalf("forfeit terminal = %+v, want classification forfeit, winner p1", forfeit)
	}
	if forfeit.Outcome != "p1_win" {
		t.Errorf("forfeit outcome = %q, want \"p1_win\"", forfeit.Outcome)
	}
	natural := gameOverTerminal(t, &Agent{mc: &bots.MatchContext{Seat: 0}},
		bots.MatchResult{Classification: "natural", Outcome: "p0_win", Winner: "p0"})
	if natural.Outcome != "p0_win" {
		t.Errorf("natural outcome = %q, want \"p0_win\"", natural.Outcome)
	}
}

// slowAckBot takes its time over game_over, then records the line and answers.
const slowAckBot = `import json, os, sys, time
for line in sys.stdin:
    request = json.loads(line)
    reply = {"protocol": "spellbench/v2", "request_id": request["request_id"], "response_type": "ack"}
    if request["request_type"] == "hello":
        reply.update(response_type="hello_ok", bot={"name": "slow", "version": "1.0.0"})
    elif request["request_type"] == "game_over":
        time.sleep(0.2)
        with open(os.environ["SPELLBENCH_RECORD"], "w") as record:
            record.write(line)
    print(json.dumps(reply), flush=True)
`

func TestGameOverReachesASlowAgent(t *testing.T) {
	bot := filepath.Join(t.TempDir(), "slow_ack.py")
	if err := os.WriteFile(bot, []byte(slowAckBot), 0o755); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "game_over.json")
	t.Setenv("SPELLBENCH_RECORD", record)
	e, _, _ := testEngine(t)
	agent, _ := attachTestSeatArgv(t, e, 0, []string{python(t), bot}, 0)
	agent.EndMatch(bots.MatchResult{Classification: "natural", Outcome: "p0_win", Winner: "p0"})
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the agent was stopped before it finished with game_over: %v", err)
	}
	var req gameOverReq
	if err := json.Unmarshal(raw, &req); err != nil || req.RequestType != "game_over" || req.Terminal.Outcome != "p0_win" {
		t.Fatalf("recorded %q: %v", raw, err)
	}
}

// mutedBot answers the handshake and then nothing.
const mutedBot = `import json, sys
for line in sys.stdin:
    request = json.loads(line)
    reply = {"protocol": "spellbench/v2", "request_id": request["request_id"], "response_type": "ack"}
    if request["request_type"] == "hello":
        reply.update(response_type="hello_ok", bot={"name": "muted", "version": "1.0.0"})
    if request["request_type"] in ("hello", "game_start"):
        print(json.dumps(reply), flush=True)
`

// mutedWrapper starts mutedBot under a shell that stays alive as its parent, so
// killing the agent's process leaves the bot holding the pipes.
func mutedWrapper(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	bot, wrapper := filepath.Join(dir, "muted.py"), filepath.Join(dir, "muted.sh")
	src := "#!/bin/sh\n" + python(t) + " " + bot + "\nexit 0\n"
	if err := os.WriteFile(bot, []byte(mutedBot), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrapper, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{wrapper}
}

func TestGameOverWaitIsBounded(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, _ := attachTestSeatArgv(t, e, 0, mutedWrapper(t), 0)
	start := time.Now()
	agent.EndMatch(bots.MatchResult{Classification: "halted"})
	if d := time.Since(start); d < gameOverGrace || d > gameOverGrace+5*time.Second {
		t.Fatalf("EndMatch returned after %v with a muted agent, want the %v grace", d, time.Duration(gameOverGrace))
	}
}

func TestGameOverWaitEndsWithTheMatchContext(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, _ := attachTestSeatArgv(t, e, 0, mutedWrapper(t), 0)
	agent.cancel()
	start := time.Now()
	agent.EndMatch(bots.MatchResult{Classification: "halted"})
	if d := time.Since(start); d >= gameOverGrace {
		t.Fatalf("EndMatch took %v after the match context ended", d)
	}
}
