package gorged

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const botOptsSpyPolicy = "botopts-spy"

var (
	botOptsSpyOnce   sync.Once
	botOptsSpyMu     sync.Mutex
	botOptsSpyConfig bots.Options
)

func registerBotOptsSpy(t *testing.T) {
	t.Helper()
	botOptsSpyOnce.Do(func() {
		bots.Register(bots.Entry{
			Info: bots.Info{
				Name:        botOptsSpyPolicy,
				Label:       "test-only option spy",
				Tier:        bots.Experimental,
				Description: "test-only entry for -bot-opt/-bot-config resolution",
				Env:         true,
				Formats:     []string{"constructed", "commander"},
				MaxSeats:    2,
				Options: []bots.OptionSpec{
					{Key: "precedence", Type: "string", Default: "default", Help: "string source order"},
					{Key: "count", Type: "int", Default: "1", Help: "integer option"},
					{Key: "ratio", Type: "float", Default: "0.5", Help: "float option"},
					{Key: "enabled", Type: "bool", Default: "false", Help: "bool option"},
				},
			},
			New: func(o bots.Options) (seat.Seat, error) {
				botOptsSpyMu.Lock()
				botOptsSpyConfig = o
				botOptsSpyMu.Unlock()
				return heuristic.NewBot(o.Seed), nil
			},
		})
	})
	if _, ok := bots.Lookup(botOptsSpyPolicy); !ok {
		t.Fatal("the bot-options spy entry was not registered")
	}
}

func emptyEnv(string) string { return "" }

func TestBotOptPrecedence(t *testing.T) {
	registerBotOptsSpy(t)
	path := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(path, []byte(`{"precedence":"file","count":2,"ratio":0.25,"enabled":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := func(name string) string {
		switch name {
		case "GORGED_BOT_BOTOPTS_SPY_PRECEDENCE":
			return "env"
		case "GORGED_BOT_BOTOPTS_SPY_RATIO":
			return "0.75"
		}
		return ""
	}
	got, err := resolveBotOptions(
		[]string{botOptsSpyPolicy + ".precedence=flag"},
		[]string{botOptsSpyPolicy + "=" + path},
		env,
	)
	if err != nil {
		t.Fatalf("resolveBotOptions: %v", err)
	}
	values := got[botOptsSpyPolicy]
	for key, want := range map[string]string{
		"precedence": "flag",
		"count":      "2",
		"ratio":      "0.75",
		"enabled":    "false",
	} {
		if got := values[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestBotOptUnknownKeyFails(t *testing.T) {
	registerBotOptsSpy(t)
	_, err := resolveBotOptions([]string{botOptsSpyPolicy + ".nope=1"}, nil, emptyEnv)
	if err == nil {
		t.Fatal("resolveBotOptions accepted an unknown key")
	}
	for _, want := range []string{"-bot-opt", botOptsSpyPolicy, "nope"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err, want)
		}
	}
}

func TestBotOptWrongTypeFails(t *testing.T) {
	registerBotOptsSpy(t)
	_, err := resolveBotOptions([]string{botOptsSpyPolicy + ".count=not-an-int"}, nil, emptyEnv)
	if err == nil {
		t.Fatal("resolveBotOptions accepted an int option with a non-int value")
	}
	for _, want := range []string{"-bot-opt", "count", "int"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err, want)
		}
	}
}

func TestBotOptRequiredMissingFails(t *testing.T) {
	registerBotOptsSpy(t)
	entries := []bots.Entry{{
		Info: bots.Info{
			Name:    "required-spy",
			Tier:    bots.Experimental,
			Options: []bots.OptionSpec{{Key: "required", Type: "string", Required: true}},
		},
		New: func(bots.Options) (seat.Seat, error) { return nil, nil },
	}}
	_, err := resolveBotOptionsFor(entries, nil, nil, emptyEnv)
	if err == nil {
		t.Fatal("resolveBotOptions accepted a missing required option")
	}
	for _, want := range []string{"requires option", "required"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err, want)
		}
	}
}

func TestBotOptValueReachesFactory(t *testing.T) {
	registerBotOptsSpy(t)
	got, err := resolveBotOptions([]string{botOptsSpyPolicy + ".precedence=from-flag"}, nil, emptyEnv)
	if err != nil {
		t.Fatalf("resolveBotOptions: %v", err)
	}
	if _, err := bots.New(botOptsSpyPolicy, bots.Options{Config: got[botOptsSpyPolicy]}); err != nil {
		t.Fatalf("bots.New: %v", err)
	}
	botOptsSpyMu.Lock()
	seen := botOptsSpyConfig
	botOptsSpyMu.Unlock()
	if got := seen.String("precedence"); got != "from-flag" {
		t.Errorf("factory String(precedence) = %q, want from-flag", got)
	}
	if got := seen.Int("count"); got != 1 {
		t.Errorf("factory Int(count) = %d, want declared default 1", got)
	}
	if got := seen.Float("ratio"); got != 0.5 {
		t.Errorf("factory Float(ratio) = %v, want declared default 0.5", got)
	}
	if got := seen.Bool("enabled"); got {
		t.Errorf("factory Bool(enabled) = %v, want declared default false", got)
	}
}

func TestHelpListsBotOptions(t *testing.T) {
	registerBotOptsSpy(t)
	fs := ServeFlagSetFor(Extensions{})
	var out bytes.Buffer
	fs.SetOutput(&out)
	if err := fs.Parse([]string{"-help"}); err != flag.ErrHelp {
		t.Fatalf("Parse(-help) = %v, want flag.ErrHelp", err)
	}
	text := out.String()
	for _, want := range []string{
		botOptsSpyPolicy + ".precedence (string",
		`default "default"`,
		botOptsSpyPolicy + ".count (int",
		botOptsSpyPolicy + ".ratio (float",
		botOptsSpyPolicy + ".enabled (bool",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("-help output does not contain %q\n%s", want, text)
		}
	}
}

func TestBotOptConfigReachesHostedSeat(t *testing.T) {
	registerBotOptsSpy(t)
	names, decks := testutil.SampleDecks(t, 2)
	load := func(name string) (host.Deck, error) {
		for i, n := range names {
			if n == name {
				return host.Deck{Name: n, Cards: decks[i]}, nil
			}
		}
		return host.Deck{}, fmt.Errorf("unknown sample deck %q", name)
	}
	botOptsSpyMu.Lock()
	botOptsSpyConfig = bots.Options{}
	botOptsSpyMu.Unlock()
	run := host.RunContractTable(t, host.Options{
		LoadDeck:  load,
		Sleep:     func(time.Duration, <-chan struct{}) {},
		BotConfig: map[string]map[string]string{botOptsSpyPolicy: {"precedence": "host-config"}},
	}, host.TableConfig{
		ID: "bot-options", Name: "bot-options", Seats: 2, Decks: names, Seed: 11,
		BotPolicy: botOptsSpyPolicy, AllowDecklistBots: true, Spectator: view.Omniscient,
	})
	if run.Info.State != protocol.MatchFinished {
		t.Fatalf("match state %q (reason %q), want finished", run.Info.State, run.Reason)
	}
	botOptsSpyMu.Lock()
	seen := botOptsSpyConfig
	botOptsSpyMu.Unlock()
	if got := seen.String("precedence"); got != "host-config" {
		t.Errorf("hosted factory String(precedence) = %q, want host-config", got)
	}
}
