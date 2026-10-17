package gorged

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// closedPolicies is every closed-list az policy gorged serves.
var closedPolicies = []string{policyAZC4, policyAZC5}

var closedAZOnce = map[string]*sync.Once{policyAZC4: {}, policyAZC5: {}}

// withClosedAZ returns c with policy's flags set to a.
func withClosedAZ(c config, policy string, a closedAZ) config {
	switch policy {
	case policyAZC4:
		c.az4 = a
	case policyAZC5:
		c.az5 = a
	default:
		panic(policy)
	}
	return c
}

// enableClosed registers policy through gorged's own flag path (a synthetic
// network, a two-deck belief directory), once per test binary.
func enableClosed(t *testing.T, policy string) {
	t.Helper()
	closedAZOnce[policy].Do(func() {
		dir, err := os.MkdirTemp("", "gorged-"+policy+"-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)
		path, err := writeStubNet(dir, "net.dzgw")
		if err != nil {
			t.Fatal(err)
		}
		belief := filepath.Join(dir, "belief")
		if err := os.Mkdir(belief, 0o755); err != nil {
			t.Fatal(err)
		}
		writeDeck(t, belief, "b1", "")
		writeDeck(t, belief, "b2", "")
		c := withClosedAZ(config{autoMana: true, botAutoPayMana: true}, policy, closedAZ{net: path, decks: dir, belief: belief})
		if err := c.enableClosedAZ(testutil.CorpusRegistry(t)); err != nil {
			t.Fatal(err)
		}
	})
	if _, ok := bots.Lookup(policy); !ok {
		t.Fatalf("%s is not registered", policy)
	}
}

// Each closed-list policy's flags: the network needs its decks and its
// belief (closed lists are the point), never -allow-decklist-bots; every
// refusal happens before anything registers. The az-c4 messages are the ones
// it has always printed.
func TestClosedAZFlagsValidate(t *testing.T) {
	for _, policy := range closedPolicies {
		p := "-" + policy
		ok := withClosedAZ(config{autoMana: true, botAutoPayMana: true}, policy, closedAZ{net: "n.dzgw", decks: "d", belief: "b"})
		if err := ok.checkClosedAZFlags(); err != nil {
			t.Fatalf("%s: a complete configuration without -allow-decklist-bots: %v", policy, err)
		}
		cases := []struct {
			name string
			mut  func(*closedAZ, *config)
			want string
		}{
			{"decks without a network", func(a *closedAZ, _ *config) { a.net = "" }, "need " + p + "-net"},
			{"belief without a network", func(a *closedAZ, _ *config) { a.net, a.decks = "", "" }, "need " + p + "-net"},
			{"network without decks", func(a *closedAZ, _ *config) { a.decks = "" }, "needs " + p + "-decks"},
			{"network without a belief", func(a *closedAZ, _ *config) { a.belief = "" }, "needs " + p + "-belief"},
			{"belief pool without a belief", func(a *closedAZ, _ *config) { a.belief, a.beliefPool = "", "pool.tsv" }, p + "-belief-pool needs " + p + "-belief"},
			{"no -auto-mana", func(_ *closedAZ, c *config) { c.autoMana = false }, "-auto-mana"},
			{"no -bot-auto-mana", func(_ *closedAZ, c *config) { c.botAutoPayMana = false }, "-bot-auto-mana"},
		}
		for _, tc := range cases {
			cfg := ok
			a := cfg.closedAZ(policy)
			tc.mut(&a, &cfg)
			cfg = withClosedAZ(cfg, policy, a)
			if err := cfg.checkClosedAZFlags(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s %s: error %v, want one containing %q", policy, tc.name, err, tc.want)
			}
		}
	}
	if err := (config{}).checkClosedAZFlags(); err != nil {
		t.Errorf("no closed-list flags: %v", err)
	}
}

// The flags are registered under each policy's own prefix, the demo table on
// by default.
func TestClosedAZFlagsRegister(t *testing.T) {
	fs, c := serveFlags()
	if err := fs.Parse([]string{"-az-c5-net", "c5.dzgw", "-az-c5-decks", "d5", "-az-c5-belief", "b5", "-az-c5-belief-pool", "p5",
		"-az-c4-net", "c4.dzgw", "-az-c4-demo-table=false"}); err != nil {
		t.Fatal(err)
	}
	a5, a4 := c.closedAZ(policyAZC5), c.closedAZ(policyAZC4)
	if a5.net != "c5.dzgw" || a5.decks != "d5" || a5.belief != "b5" || a5.beliefPool != "p5" || !a5.demoTable {
		t.Errorf("az-c5 flags %+v", a5)
	}
	if a4.net != "c4.dzgw" || a4.decks != "" || a4.demoTable {
		t.Errorf("az-c4 flags %+v", a4)
	}
}

// A closed-list policy does not read decklists: it is in the default offered
// set, and the vs-bot flow seats a human against it, explicitly or through a
// deck of its pool, dealing both seats from that pool.
func TestCreateGameSeatsAHumanAgainstAClosedPolicy(t *testing.T) {
	for _, policy := range closedPolicies {
		enableClosed(t, policy)
		offered, err := botPolicyOffered("")
		if err != nil {
			t.Fatal(err)
		}
		if !containsString(offered, policy) {
			t.Fatalf("offered set %v lacks %s", offered, policy)
		}
		r, gate := freshGameLock(t)
		c := config{autoMana: true, botAutoPayMana: true, botPolicy: host.BotPolicy,
			policyPools: map[string][]string{policy: {"a", "b"}}}
		create := c.createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)
		for _, req := range []httpapi.CreateGameOptions{
			{Format: host.FormatConstructed, BotPolicy: policy},
			{Format: host.FormatConstructed, HumanDeck: "a"},
		} {
			resp, err := create(req)
			if err != nil {
				t.Fatalf("%s %+v: %v", policy, req, err)
			}
			for _, d := range waitSeatDecks(t, r, resp.Table) {
				if d != "a" && d != "b" {
					t.Fatalf("%s %+v: dealt %q, outside its pool", policy, req, d)
				}
			}
		}
		// The pool stays reserved: another policy may not deal from it.
		if _, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: host.BotPolicy, HumanDeck: "a"}); err == nil ||
			!strings.Contains(err.Error(), `reserved for bot policy "`+policy+`"`) {
			t.Fatalf("an ordinary game took a deck reserved for %s: %v", policy, err)
		}
	}
}

// A deck directory shared by az-c3, az-c4 and az-c5 is one pool reserved for
// all three: catalogued once, picked for a closed-list policy by a request
// that names no policy (az-c3 cannot seat a human), still refused for an
// explicit az-c3 pick.
func TestReservedPoliciesShareADeckDirectory(t *testing.T) {
	enableAZ(t)
	enableClosed(t, policyAZC4)
	enableClosed(t, policyAZC5)
	reg := testutil.CorpusRegistry(t)
	dir := t.TempDir()
	writeDeck(t, dir, "FDN_a", "")
	writeDeck(t, dir, "FDN_b", "")
	c := config{az3Decks: dir, az4: closedAZ{decks: dir + string(filepath.Separator)}, az5: closedAZ{decks: dir}}
	az3, cat, err := c.azc3Pool(reg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	prior := []reservedDir{{policy: policyAZC3, dir: c.az3Decks, pool: az3}}
	az4, cat4, err := c.closedAZ(policyAZC4).loadPool(reg, nil, nil, prior, cat)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(az3, az4) || len(cat4) != 2 {
		t.Fatalf("pools %v / %v, catalogue %d: want one shared pool catalogued once", az3, az4, len(cat4))
	}
	if cat[0].Pool != policyAZC3 {
		t.Fatalf("loadPool wrote through to its catalogue argument: pool %q", cat[0].Pool)
	}
	for _, d := range cat4 {
		if d.Pool != "az-c3,az-c4" || d.Archetype != "test" {
			t.Errorf("deck %s pool %q archetype %q after az-c4", d.ID, d.Pool, d.Archetype)
		}
	}
	prior = append(prior, reservedDir{policy: policyAZC4, dir: c.az4.decks, pool: az4})
	az5, cat5, err := c.closedAZ(policyAZC5).loadPool(reg, nil, nil, prior, cat4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(az3, az5) || len(cat5) != 2 {
		t.Fatalf("pools %v / %v, catalogue %d: want one shared pool catalogued once", az3, az5, len(cat5))
	}
	for _, d := range cat5 {
		if d.Pool != "az-c3,az-c4,az-c5" || d.Archetype != "test" {
			t.Errorf("deck %s pool %q archetype %q after az-c5", d.ID, d.Pool, d.Archetype)
		}
		for _, p := range []string{policyAZC3, policyAZC4, policyAZC5} {
			if !httpapi.PoolHas(d.Pool, p) {
				t.Errorf("deck %s pool %q lacks %s", d.ID, d.Pool, p)
			}
		}
	}
	c.policyPools = map[string][]string{policyAZC3: az3, policyAZC4: az4, policyAZC5: az5}
	if got := c.reservedPick("FDN_a"); got != policyAZC4 {
		t.Errorf("a shared deck picks %q, want az-c4 (the first owner that may play a human)", got)
	}
	if got := c.reservedFor("FDN_a"); got != "az-c3, az-c4, az-c5" {
		t.Errorf("reservedFor %q", got)
	}
	// Two directories that both hold an id are ambiguous: refused, naming
	// the earlier pool's flag.
	other := t.TempDir()
	writeDeck(t, other, "FDN_a", "")
	if _, _, err := (closedAZ{policy: policyAZC4, decks: other}).loadPool(reg, nil, nil, prior[:1], cat); err == nil || !strings.Contains(err.Error(), "-az-c3-decks") {
		t.Errorf("az-c4: an id in both directories: error %v", err)
	}
	if _, _, err := (closedAZ{policy: policyAZC5, decks: other}).loadPool(reg, nil, nil, prior[1:], cat4); err == nil ||
		!strings.Contains(err.Error(), "-az-c5-decks") || !strings.Contains(err.Error(), "-az-c4-decks") {
		t.Errorf("az-c5: an id in az-c4's directory and its own: error %v", err)
	}
	// A closed-list pool alone: its decks are catalogued for it only.
	solo, catSolo, err := (closedAZ{policy: policyAZC5, decks: other}).loadPool(reg, nil, nil, nil, nil)
	if err != nil || len(solo) != 1 || len(catSolo) != 1 || catSolo[0].Pool != policyAZC5 {
		t.Errorf("az-c5 alone: pool %v catalogue %+v, %v", solo, catSolo, err)
	}
	// A deck with the default archetype names every owner; one with its own
	// ("test", above) keeps it.
	def := []httpapi.DeckInfo{{ID: "FDN_a", Pool: policyAZC3, Archetype: "FDN limited, plays vs az-c3"}}
	_, def4, err := c.closedAZ(policyAZC4).loadPool(reg, nil, nil, prior[:1], def)
	if err != nil || def4[0].Archetype != "FDN limited, plays vs az-c3 or az-c4" {
		t.Errorf("default archetype after az-c4: %+v, %v", def4, err)
	}
	_, def5, err := c.closedAZ(policyAZC5).loadPool(reg, nil, nil, prior, def4[:1])
	if err != nil || def5[0].Archetype != "FDN limited, plays vs az-c3, az-c4 or az-c5" {
		t.Errorf("default archetype after az-c5: %+v, %v", def5, err)
	}
	// The loader resolves a shared id from the shared directory.
	load := deckLoaders(reg, t.TempDir(), dir, az3)
	for _, a := range c.closedAZs() {
		load = reservedLoader(reg, load, a.decks, c.policyPools[a.policy])
	}
	if d, err := load("FDN_b"); err != nil || len(d.Cards) != 40 {
		t.Errorf("load(FDN_b) = %d cards, %v", len(d.Cards), err)
	}
	// Every policy has its reserved pool (draft events still leave out az-c3,
	// which reads decklists: eventPolicies).
	var reserved []string
	for p := range c.policyPools {
		reserved = append(reserved, p)
	}
	sort.Strings(reserved)
	if got := reserved; !reflect.DeepEqual(got, []string{policyAZC3, policyAZC4, policyAZC5}) {
		t.Errorf("reserved policies %v", got)
	}
}

// Each closed-list policy's demo table is a bot-only table that needs no
// decklist opt-in.
func TestClosedAZDemoTables(t *testing.T) {
	for _, policy := range closedPolicies {
		c := withClosedAZ(config{autoMana: true, botAutoPayMana: true}, policy, closedAZ{net: "n", demoTable: true})
		tc := c.closedAZ(policy).table(c, "t3", 7, []string{"a", "b"}, view.Public)
		if tc == nil || tc.BotPolicy != policy || tc.AllowDecklistBots || len(tc.Humans) != 0 || tc.ID != "t3" || tc.Name != policy+" vs "+policy+" (FDN)" {
			t.Fatalf("%s demo table %+v", policy, tc)
		}
		c = withClosedAZ(c, policy, closedAZ{net: "n"})
		if c.closedAZ(policy).table(c, "t3", 7, []string{"a"}, view.Public) != nil {
			t.Fatalf("-%s-demo-table=false still built the table", policy)
		}
	}
}
