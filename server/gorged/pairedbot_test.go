package gorged

import (
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestPairedBotFlags(t *testing.T) {
	_, c := serveFlags()
	if c.pairedBot != "" {
		t.Fatalf("-paired-bot defaults to %q, want off", c.pairedBot)
	}
	reg := testutil.CorpusRegistry(t)
	deck := testutil.RepoDeck(t, reg, "mono-red-goblins")
	load := func(string) (host.Deck, error) { return host.Deck{Name: "d", Cards: deck}, nil }
	pool := []string{"d"}

	cfg := config{engine: "gorge", pairedBotSims: 8, pairedBotSearches: 1}
	if e, err := cfg.pairedBotEntry(reg, load, pool); err != nil || e != nil {
		t.Fatalf("off: %+v, %v", e, err)
	}
	cfg.pairedBot = host.PairedBotOff
	if e, err := cfg.pairedBotEntry(reg, load, pool); err != nil || e != nil {
		t.Fatalf("-paired-bot off: %+v, %v", e, err)
	}
	cfg.pairedBot = pairedBotBuiltin
	e, err := cfg.pairedBotEntry(reg, load, pool)
	if err != nil || e == nil || !e.ClosedLists {
		t.Fatalf("built-in: %+v, %v", e, err)
	}
	if _, err := host.New(host.Options{LoadDeck: load, PairedBot: e}); err != nil {
		t.Fatalf("the host refused the built-in: %v", err)
	}
	refused := func(name string, mutate func(*config), want string) {
		t.Helper()
		bad := cfg
		mutate(&bad)
		_, err := bad.pairedBotEntry(reg, load, pool)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want %q", name, err, want)
		}
	}
	refused("unknown bot", func(c *config) { c.pairedBot = "no-such-bot" }, "unknown bot")
	refused("another engine", func(c *config) { c.engine = "dirac" }, "needs -engine")
	refused("no sims", func(c *config) { c.pairedBotSims = 0 }, "-paired-bot-sims")
	refused("no searches", func(c *config) { c.pairedBotSearches = 0 }, "-paired-bot-searches")
	if _, err := cfg.pairedBotEntry(reg, load, nil); err == nil {
		t.Error("a built-in with no decks to guess from was accepted")
	}
}

// A registered policy that reads real lists gets past the name lookup and is
// refused by the host, which checks a built seat.
func TestPairedBotRefusesAnOpenListPolicy(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deck := testutil.RepoDeck(t, reg, "mono-red-goblins")
	load := func(string) (host.Deck, error) { return host.Deck{Name: "d", Cards: deck}, nil }
	cfg := config{engine: "gorge", pairedBot: "az-redeal", pairedBotSearches: 1}
	e, err := cfg.pairedBotEntry(reg, load, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.New(host.Options{LoadDeck: load, PairedBot: e}); err == nil || !strings.Contains(err.Error(), "real decklists") {
		t.Fatalf("az-redeal as a paired bot: %v", err)
	}
}

// A table may choose what the server offers, and nothing else. A server
// without -paired-bot offers nothing to choose.
func TestCreateGameChoosesAPairedBot(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deck := testutil.RepoDeck(t, reg, "mono-red-goblins")
	load := func(string) (host.Deck, error) { return host.Deck{Name: "d", Cards: deck}, nil }
	builtin, err := (config{engine: "gorge", pairedBot: pairedBotBuiltin, pairedBotSims: 4, pairedBotSearches: 1}).pairedBotEntry(reg, load, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	var gate *seatGate
	build := func(def *bots.Entry) func(httpapi.CreateGameOptions) (httpapi.CreateGameResponse, error) {
		r, err := host.New(host.Options{LoadDeck: load, Sleep: func(time.Duration, <-chan struct{}) {}, PairedBot: def, PairedBotSearches: 1})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		gate = &seatGate{tokenToClaim: map[string]httpapi.SeatClaim{}, claimTokens: map[httpapi.SeatClaim]string{}}
		return (config{mulligans: 0, autoMana: true, botAutoPayMana: true}).createGame(r, gate, nil, []string{"a", "b"}, view.Public)
	}
	create := build(builtin)
	for _, name := range []string{"", host.PairedBotOff, pairedBotBuiltin} {
		if _, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, PairedBot: name}); err != nil {
			t.Errorf("choice %q: %v", name, err)
		}
	}
	minted := len(gate.tokenToClaim)
	if _, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, PairedBot: "no-such-bot"}); err == nil || !strings.Contains(err.Error(), "not offered") {
		t.Errorf("an unoffered bot: %v", err)
	}
	if len(gate.tokenToClaim) != minted {
		t.Error("a refused request left a seat token behind")
	}
	if _, err := build(nil)(httpapi.CreateGameOptions{Format: host.FormatConstructed, PairedBot: pairedBotBuiltin}); err == nil || !strings.Contains(err.Error(), "not offered") {
		t.Errorf("a server without the option: %v", err)
	}
}

// The built-in search is on offer beside a registered default, and only with a
// default: a choice cannot switch searching on.
func TestPairedBotChoicesNeedTheFlag(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deck := testutil.RepoDeck(t, reg, "mono-red-goblins")
	load := func(string) (host.Deck, error) { return host.Deck{Name: "d", Cards: deck}, nil }
	c := config{engine: "gorge", pairedBot: pairedBotBuiltin, pairedBotSims: 4, pairedBotSearches: 1}
	if got := c.pairedBotChoices(host.Options{}, reg, load, []string{"a"}); got != nil {
		t.Errorf("no default: %v", got)
	}
	def, err := c.pairedBotEntry(reg, load, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range c.pairedBotChoices(host.Options{LoadDeck: load, PairedBot: def}, reg, load, []string{"a"}) {
		if e.Name == pairedBotBuiltin {
			t.Error("the default is listed as its own choice")
		}
	}
	other := *def
	other.Name = "another"
	got := c.pairedBotChoices(host.Options{LoadDeck: load, PairedBot: &other}, reg, load, []string{"a"})
	if len(got) == 0 || got[0].Name != pairedBotBuiltin {
		t.Errorf("beside another default: %v", got)
	}
}
