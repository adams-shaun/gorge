package gorged

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// botOptsOnePolicy is the directive's replacement fixture: one declared
// option, observed at the factory, playable through the host.
const botOptsOnePolicy = "botopts-one"

var (
	botOptsOneOnce sync.Once
	botOptsOneMu   sync.Mutex
	botOptsOneSeen []bots.Options
)

func registerBotOptsOne(t *testing.T) {
	t.Helper()
	botOptsOneOnce.Do(func() {
		bots.Register(bots.Entry{
			Info: bots.Info{
				Name:        botOptsOnePolicy,
				Label:       "test-only one-option spy",
				Tier:        bots.Experimental,
				Description: "test-only entry for the -bot-opt hosted-game path",
				Formats:     []string{"constructed", "commander"},
				MaxSeats:    2,
				Options: []bots.OptionSpec{{
					Key:     "greeting",
					Type:    "string",
					Default: "default",
					Help:    "value observed at the factory",
				}},
			},
			New: func(o bots.Options) (seat.Seat, error) {
				botOptsOneMu.Lock()
				botOptsOneSeen = append(botOptsOneSeen, o)
				botOptsOneMu.Unlock()
				return heuristic.NewBot(o.Seed), nil
			},
		})
	})
	if _, ok := bots.Lookup(botOptsOnePolicy); !ok {
		t.Fatal("the one-option spy entry was not registered")
	}
}

func resetBotOptsOne(t *testing.T) {
	t.Helper()
	botOptsOneMu.Lock()
	botOptsOneSeen = nil
	botOptsOneMu.Unlock()
	t.Cleanup(func() {
		botOptsOneMu.Lock()
		botOptsOneSeen = nil
		botOptsOneMu.Unlock()
	})
}

func botOptsOneConfigs() []bots.Options {
	botOptsOneMu.Lock()
	defer botOptsOneMu.Unlock()
	return append([]bots.Options(nil), botOptsOneSeen...)
}

func botOptsSampleLoader(t *testing.T, names []string, decks [][]*cards.Card) func(string) (host.Deck, error) {
	t.Helper()
	return func(name string) (host.Deck, error) {
		for i, n := range names {
			if n == name {
				return host.Deck{Name: n, Cards: decks[i]}, nil
			}
		}
		return host.Deck{}, fmt.Errorf("unknown sample deck %q", name)
	}
}

// TestBotOptFlagReachesHostedGame drives -bot-opt through the real flag and
// resolution path, then seats the policy at slot 0 with the built-in bot at
// slot 1. The table's BotPolicy is `bot`, so the only spy factory call is the
// per-seat policy build; its value proves seatPolicySeats got the resolved
// config, and RunContractTable proves the game finished and replayed.
func TestBotOptFlagReachesHostedGame(t *testing.T) {
	registerBotOptsOne(t)
	resetBotOptsOne(t)
	names, decks := testutil.SampleDecks(t, 2)

	fs, c := flagsFor(Extensions{})
	fs.SetOutput(io.Discard)
	t.Setenv("GORGED_BOT_BOTOPTS_ONE_GREETING", "")
	if err := fs.Parse([]string{"-bot-opt", botOptsOnePolicy + ".greeting=from-flag"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := c.resolveBotOptions(); err != nil {
		t.Fatalf("resolveBotOptions: %v", err)
	}

	run := host.RunContractTable(t, host.Options{
		LoadDeck:  botOptsSampleLoader(t, names, decks),
		Sleep:     func(time.Duration, <-chan struct{}) {},
		BotConfig: c.botConfig,
	}, host.TableConfig{
		ID:              "bot-opt-flag",
		Name:            "bot-opt-flag",
		Seats:           2,
		Decks:           names,
		Seed:            11,
		BotPolicy:       bots.Default,
		SeatBotPolicies: []string{botOptsOnePolicy, bots.Default},
		AutoMana:        true,
		BotAutoPayMana:  true,
		Spectator:       view.Omniscient,
	})
	if run.Info.State != protocol.MatchFinished {
		t.Fatalf("match state %q (reason %q), want finished", run.Info.State, run.Reason)
	}
	if len(run.Log.Intents) == 0 {
		t.Fatal("match recorded no intents")
	}
	seen := botOptsOneConfigs()
	if len(seen) != 1 {
		t.Fatalf("spy factory ran %d times, want exactly the per-seat build", len(seen))
	}
	if got := seen[0].String("greeting"); got != "from-flag" {
		t.Errorf("factory String(greeting) = %q, want from-flag", got)
	}
}

// TestBotConfigReachesCaretaker proves the human-seat caretaker is built with
// the table policy's resolved config: a 2-seat table with seat 0 human builds
// two default bots and then the caretaker, all through the spy policy, and
// every observed factory call must carry the BotConfig value.
func TestBotConfigReachesCaretaker(t *testing.T) {
	registerBotOptsOne(t)
	resetBotOptsOne(t)
	names, decks := testutil.SampleDecks(t, 2)

	r, err := host.New(host.Options{
		LoadDeck:  botOptsSampleLoader(t, names, decks),
		Sleep:     func(time.Duration, <-chan struct{}) {},
		BotConfig: map[string]map[string]string{botOptsOnePolicy: {"greeting": "caretaker"}},
	})
	if err != nil {
		t.Fatalf("host.New: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	cfg := host.TableConfig{
		ID: "bot-config-caretaker", Name: "bot-config-caretaker", Seats: 2, Decks: names, Seed: 11,
		Humans: []int{0}, BotPolicy: botOptsOnePolicy, AutoMana: true, BotAutoPayMana: true,
		Spectator: view.Omniscient, Perpetual: false,
	}
	if err := r.AddTable(cfg); err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	if err := r.Start(cfg.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Two default seats plus the caretaker; the human seat parks unanswered
	// with ThinkTimeout 0, so no more spy builds can follow.
	deadline := time.Now().Add(2 * time.Second)
	for len(botOptsOneConfigs()) < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("spy factory ran %d times, want 3 (two seats and the caretaker)", len(botOptsOneConfigs()))
		}
		time.Sleep(time.Millisecond)
	}
	seen := botOptsOneConfigs()
	if len(seen) != 3 {
		t.Fatalf("spy factory ran %d times, want 3 (two seats and the caretaker)", len(seen))
	}
	for i, o := range seen {
		if got := o.String("greeting"); got != "caretaker" {
			t.Errorf("factory call %d String(greeting) = %q, want caretaker", i, got)
		}
	}
}
