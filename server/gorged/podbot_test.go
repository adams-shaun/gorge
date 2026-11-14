package gorged

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// podBotFixture is four MBC Commander-draft decks (draftsim -set MBC -collation
// draft -format commander -deck-size 60 -structure none -players bot -events 1
// -decks-out): card names and counts only, a stand-in for make pod-bot-decks.
const podBotFixture = "testdata/pod-bot-decks"

// TestPodBotFlags: -pod-bot-decks registers az-closed-maxn over the decks it
// names, after which -events-bot-policy may name it and the paired-bot
// choices offer it; without the flag nothing is loaded, and a bad flag fails
// startup.
func TestPodBotFlags(t *testing.T) {
	_, def := serveFlags()
	if def.podBotDecks != "" || def.podBotSims != azredeal.ClosedMaxNSims {
		t.Fatalf("defaults: -pod-bot-decks %q, -pod-bot-sims %d", def.podBotDecks, def.podBotSims)
	}
	reg := testutil.CorpusRegistry(t)
	if err := (config{}).enablePodBot(reg); err != nil {
		t.Fatalf("no flag: %v", err)
	}
	for name, tc := range map[string]struct {
		c    config
		want string
	}{
		"no sims":      {config{podBotDecks: podBotFixture}, "-pod-bot-sims"},
		"no directory": {config{podBotDecks: "testdata/no-such-dir", podBotSims: 4}, "-pod-bot-decks"},
		"dirac":        {config{podBotDecks: podBotFixture, podBotSims: 4, engine: adapter.Name}, "gorge's rules engine"},
	} {
		if err := tc.c.enablePodBot(reg); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want an error naming %q", name, err, tc.want)
		}
	}

	c := config{engine: "gorge", podBotDecks: podBotFixture, podBotSims: 4}
	if err := c.enablePodBot(reg); err != nil {
		t.Fatal(err)
	}
	e, ok := bots.Lookup(azredeal.PolicyClosedMaxN)
	if !ok || !e.ClosedLists || e.MaxSeats != 4 {
		t.Fatalf("after -pod-bot-decks: %+v, %v", e.Info, ok)
	}

	// The offered set (every registered policy at the -bot-policies default)
	// holds it, so draft events may default to it.
	var err error
	if c.botPolicy, c.botPolicies, err = validateBotPolicyFlags("", ""); err != nil {
		t.Fatal(err)
	}
	c.eventsBotPolicy = azredeal.PolicyClosedMaxN
	if got, err := c.eventsDefaultPolicy(); err != nil || got != azredeal.PolicyClosedMaxN {
		t.Fatalf("-events-bot-policy %s: %q, %v", azredeal.PolicyClosedMaxN, got, err)
	}

	// The paired-bot choices beside az-closed include it, and the host takes it.
	deck := testutil.RepoDeck(t, reg, "mono-red-goblins")
	load := func(string) (host.Deck, error) { return host.Deck{Name: "d", Cards: deck}, nil }
	c.pairedBot, c.pairedBotSims, c.pairedBotSearches = pairedBotBuiltin, 4, 1
	pdef, err := c.pairedBotEntry(reg, load, []string{"a"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	o := host.Options{LoadDeck: load, PairedBot: pdef}
	var got *bots.Entry
	for _, e := range c.pairedBotChoices(o, reg, load, []string{"a"}, nil) {
		if e.Name == azredeal.PolicyClosedMaxN {
			got = e
		}
	}
	if got == nil {
		t.Fatal("az-closed-maxn is not among the paired-bot choices")
	}
	if got.MaxSeats != 4 {
		t.Errorf("paired-bot choice %+v", got.Info)
	}
}
