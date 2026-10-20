package main

import (
	"bytes"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// reasonProbe is a heuristic seat that records the MatchResult mtgsim
// reports to it.
type reasonProbe struct {
	engineseat.Seat
	got *bots.MatchResult
}

func (p reasonProbe) EndMatch(r bots.MatchResult) { *p.got = r }

func TestNaturalGameOverCarriesNoNotFinishedReason(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	var got bots.MatchResult
	bots.Register(bots.Entry{
		Info: bots.Info{Name: "mtgsim-reason-probe", Label: "probe", Tier: bots.Experimental, Formats: []string{"constructed"}, MaxSeats: 2},
		New: func(o bots.Options) (engineseat.Seat, error) {
			return reasonProbe{Seat: heuristic.NewBot(o.Seed), got: &got}, nil
		},
	})
	var buf bytes.Buffer
	if err := runAgents("", 2, 0, 1, false, false, dir, "mtgsim-reason-probe", "", nil, &buf); err != nil {
		t.Fatalf("runAgents: %v\n%s", err, buf.String())
	}
	if got.Classification != "natural" {
		t.Fatalf("classification = %q, want natural (the game must have finished)\n%s", got.Classification, buf.String())
	}
	if got.Reason == "not_finished" {
		t.Errorf("a natural game_over reports reason %q", got.Reason)
	}
}
