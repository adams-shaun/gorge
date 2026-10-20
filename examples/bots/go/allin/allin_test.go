package allin_test

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/examples/bots/go/allin"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRegistered: the init ran, and bots.New fills the declared default.
func TestRegistered(t *testing.T) {
	e, ok := bots.Lookup(allin.Name)
	if !ok {
		t.Fatalf("%s is not registered", allin.Name)
	}
	if len(e.Options) != 1 || e.Options[0].Key != "min-power" {
		t.Fatalf("options = %+v", e.Options)
	}
	if _, err := bots.New(allin.Name, bots.Options{Seed: 1}); err != nil {
		t.Fatal(err)
	}
}

// TestPlaysAgainstTheBot plays seeded games against the heuristic bot with
// testkit/bench, the engine-only runner: no host, no network, no clock. The
// sample decks are synthetic, so no card corpus is needed.
func TestPlaysAgainstTheBot(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	wins := 0
	for g := 0; g < 10; g++ {
		seed := uint64(1000 + g)
		mine, err := bots.New(allin.Name, bots.Options{Seed: seed ^ 1})
		if err != nil {
			t.Fatal(err)
		}
		seats := []seat.Seat{mine, heuristic.NewBot(seed ^ 2)}
		cfg := rules.Config{Seed: seed, Names: names, Decks: decks}
		out, _, err := bench.PlayGame(cfg, seats, 200, 20000, bench.Hooks{})
		if err != nil {
			t.Fatalf("game %d: %v", g, err)
		}
		if out.IsStalled() {
			t.Fatalf("game %d stalled: %s", g, out.StallOn)
		}
		if !out.Draw && out.WinnerSeat == 0 {
			wins++
		}
	}
	t.Logf("allin won %d of 10 against bot", wins)
}

// TestHostContract seats the policy at a real host table (the same harness
// gorge runs on its own hosted policies) and replays the finished match to
// its recorded head: the policy is deterministic and every answer it gave
// was legal.
func TestHostContract(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	run := host.RunContractTable(t, host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			for i, n := range names {
				if n == name {
					return host.Deck{Name: n, Cards: decks[i]}, nil
				}
			}
			return host.Deck{}, nil
		},
		Sleep:     func(time.Duration, <-chan struct{}) {},
		BotConfig: map[string]map[string]string{allin.Name: {"min-power": "2"}},
	}, host.TableConfig{
		ID: "allin", Name: "allin", Seats: 2, Decks: names, Seed: 7,
		BotPolicy: bots.Default, SeatBotPolicies: []string{allin.Name, bots.Default},
		Spectator: view.Public,
	})
	if run.Info.State != protocol.MatchFinished {
		t.Fatalf("state %q (%s), want finished", run.Info.State, run.Reason)
	}
}
