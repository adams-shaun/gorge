package gorged

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/host"
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
