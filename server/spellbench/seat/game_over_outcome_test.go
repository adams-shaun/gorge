package seat

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

type discardCloser struct{ io.Writer }

func (discardCloser) Close() error { return nil }

func gameOverTerminal(t *testing.T, a *Agent, r bots.MatchResult) terminalNotice {
	t.Helper()
	var sent bytes.Buffer
	a.stdin = discardCloser{&sent}
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
