package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// stalledTokenGame plays 30 Dragon Fodder + 30 Mountain a side under a small
// object cap until the cap aborts it, the shape of the hunt's seed 79231
// (Scute Swarm doubling past the default 10000-object cap).
func stalledTokenGame(t *testing.T) (*rules.Engine, rules.Config, int, *rules.LivelockError) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	fodder, ok := reg.Lookup("Dragon Fodder")
	if !ok {
		t.Fatal(`corpus card "Dragon Fodder" not found`)
	}
	mountain, ok := reg.Lookup("Mountain")
	if !ok {
		t.Fatal(`corpus card "Mountain" not found`)
	}
	var d []*cards.Card
	for i := 0; i < 30; i++ {
		d = append(d, fodder, mountain)
	}
	cfg := rules.Config{Seed: 17, Names: []string{"p0", "p1"},
		Decks:  [][]*cards.Card{d, append([]*cards.Card(nil), d...)},
		Tokens: reg.Tokens, NameUniverse: reg.Universe(),
		LoopGuard: &rules.LoopGuard{MaxObjs: 121}}
	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	seats := []engineseat.Seat{heuristic.NewBot(17 ^ 1), heuristic.NewBot(17 ^ 2)}
	n, err := driveMatch(e, seats, newSeatRoute(cfg, seats), nil)
	var lle *rules.LivelockError
	if !errors.As(err, &lle) {
		t.Fatalf("the capped game ended with %v, want a *rules.LivelockError", err)
	}
	if !rules.IsVolumeReason(lle.Reason) {
		t.Fatalf("the capped game tripped %q, want a volume reason", lle.Reason)
	}
	return e, cfg, n, lle
}

// TestBigBoardStallIsNotAFailure: a game the object cap ends is a recorded
// bigboard stall (true, no LIVELOCK marker), and -verify replays its
// recorded prefix to "replay OK" instead of failing the run.
func TestBigBoardStallIsNotAFailure(t *testing.T) {
	e, cfg, n, lle := stalledTokenGame(t)
	if len(e.L.Intents) < 2 {
		t.Fatalf("the stalled game recorded %d intents, want a prefix to replay", len(e.L.Intents))
	}

	var out bytes.Buffer
	if !reportBigBoard(&out, 17, 2, n, lle, e, cfg, true) {
		t.Fatalf("a bigboard stall counted as a failure:\n%s", out.String())
	}
	got := out.String()
	if strings.Contains(got, "LIVELOCK") {
		t.Errorf("the stall line carries the LIVELOCK failure marker:\n%s", got)
	}
	for _, want := range []string{"BIGBOARD STALL", "object cap", "replay OK"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}

// TestBigBoardStallVerifyCatchesAPrefixDivergence: the verify is a real
// comparison -- a prefix replayed under a different seed's log fails.
func TestBigBoardStallVerifyCatchesAPrefixDivergence(t *testing.T) {
	e, cfg, n, lle := stalledTokenGame(t)
	// Corrupt one recorded event of the prefix; the replay must report it.
	e.L.Events[len(e.L.Events)/2].Amount += 7

	var out bytes.Buffer
	if reportBigBoard(&out, 17, 2, n, lle, e, cfg, true) {
		t.Fatalf("a corrupted prefix still verified:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "replay diverged") {
		t.Errorf("output lacks a divergence line:\n%s", out.String())
	}
}
