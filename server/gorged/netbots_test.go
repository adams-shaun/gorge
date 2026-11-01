package gorged

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// closedPolicies is every stub policy that guesses opponents' lists.
var closedPolicies = []string{testClosedA, testClosedB}

var (
	enableOpenOnce   sync.Once
	enableClosedOnce = map[string]*sync.Once{testClosedA: {}, testClosedB: {}}
)

// withNet returns c with policy's flags set to a.
func withNet(c config, policy string, a netPolicy) config {
	c.nets = append(append([]netPolicy(nil), c.nets...), stubPolicy(policy, a))
	return c
}

// netOf returns policy's entry in c.
func netOf(c config, policy string) netPolicy {
	for _, a := range c.nets {
		if a.policy == policy {
			return a
		}
	}
	panic("gorged: no net policy " + policy)
}

// enableOpen registers the decklist-reading stub over a synthetic network, once
// per test binary.
func enableOpen(t *testing.T) {
	t.Helper()
	enableOpenOnce.Do(func() {
		dir := t.TempDir()
		path, err := writeStubNet(dir, "net.dzgw")
		if err != nil {
			t.Fatal(err)
		}
		c := withNet(config{allowDecklistBots: true, autoMana: true, botAutoPayMana: true}, testOpenPolicy, netPolicy{net: path, decks: dir})
		if err := c.enableNetBots(nil); err != nil {
			t.Fatal(err)
		}
	})
	if _, ok := bots.Lookup(testOpenPolicy); !ok {
		t.Fatalf("%s is not registered", testOpenPolicy)
	}
}

// enableClosed registers policy through gorged's own flag path (a synthetic
// network, a two-deck belief directory), once per test binary.
func enableClosed(t *testing.T, policy string) {
	t.Helper()
	enableClosedOnce[policy].Do(func() {
		dir := t.TempDir()
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
		c := withNet(config{autoMana: true, botAutoPayMana: true}, policy, netPolicy{net: path, decks: dir, belief: belief})
		if err := c.enableNetBots(testutil.CorpusRegistry(t)); err != nil {
			t.Fatal(err)
		}
	})
	if _, ok := bots.Lookup(policy); !ok {
		t.Fatalf("%s is not registered", policy)
	}
}

// A binary with no registered net bot has none of their flags: naming one is
// the flag package's ordinary unknown-flag error.
func TestNoRegisteredNetBotNoFlags(t *testing.T) {
	netBotsMu.Lock()
	saved := netBots
	netBots = nil
	netBotsMu.Unlock()
	t.Cleanup(func() {
		netBotsMu.Lock()
		netBots = saved
		netBotsMu.Unlock()
	})
	fs := ServeFlagSet()
	if err := fs.Parse([]string{"-" + testOpenPolicy + "-net", "n.dzgw"}); err == nil || !strings.Contains(err.Error(), "not defined") {
		t.Errorf("error %v, want an unknown-flag error", err)
	}
	fs.VisitAll(func(f *flag.Flag) {
		if strings.HasSuffix(f.Name, "-belief") || strings.HasSuffix(f.Name, "-net") {
			t.Errorf("flag -%s with no net bot registered", f.Name)
		}
	})
}

// A decklist-reading policy's flags come as a pair and need what it needs; a
// refused configuration registers nothing.
func TestOpenNetFlagsValidate(t *testing.T) {
	const policy = "net-open-refused"
	sb := &stubNetBot{info: stubInfo(policy, false)}
	withRegisteredNetBot(t, policy, NetBot{NetHelp: "its test network", ReadsDecklists: true, Enable: sb.enable})
	ok := config{vsbot: true, allowDecklistBots: true, autoMana: true, botAutoPayMana: true}
	full := netPolicy{net: "n.dzgw", decks: "d"}
	p := "-" + policy
	cases := []struct {
		name string
		mut  func(*netPolicy, *config)
		want string
	}{
		{"decks without a network", func(a *netPolicy, _ *config) { a.net = "" }, "need " + p + "-net"},
		{"network without decks", func(a *netPolicy, _ *config) { a.decks = "" }, "needs " + p + "-decks"},
		{"no -allow-decklist-bots", func(_ *netPolicy, c *config) { c.allowDecklistBots = false }, "needs -allow-decklist-bots"},
		{"no -auto-mana", func(_ *netPolicy, c *config) { c.autoMana = false }, "-auto-mana"},
		{"no -bot-auto-mana", func(_ *netPolicy, c *config) { c.botAutoPayMana = false }, "-bot-auto-mana"},
		{"belief without a network", func(a *netPolicy, _ *config) { a.net, a.decks, a.belief = "", "", "b" }, "need " + p + "-net"},
		{"belief pool without a belief", func(a *netPolicy, _ *config) { a.beliefPool = "pool.tsv" }, p + "-belief-pool needs " + p + "-belief"},
	}
	for _, tc := range cases {
		cfg, a := ok, full
		tc.mut(&a, &cfg)
		err := withNet(cfg, policy, a).enableNetBots(nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one containing %q", tc.name, err, tc.want)
		}
	}
	if err := withNet(config{}, policy, netPolicy{}).enableNetBots(nil); err != nil {
		t.Errorf("no flags: %v", err)
	}
	// A network the loader refuses is a startup error, not a half-served policy.
	bad := filepath.Join(t.TempDir(), "bad.dzgw")
	if err := os.WriteFile(bad, []byte("not a network"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := withNet(ok, policy, netPolicy{net: bad, decks: "d"}).enableNetBots(nil); err == nil {
		t.Error("a bad network file was accepted")
	}
	if _, found := bots.Lookup(policy); found {
		t.Error("a refused configuration left the policy registered")
	}
}

func openConfig() config {
	return config{mulligans: 0, autoMana: true, botAutoPayMana: true,
		policyPools: map[string][]string{testOpenPolicy: {"a", "b"}}}
}

// A policy that reads opponents' decklists is refused by the vs-bot flow, which — which always seats a human — refuses it, however it is
// picked: explicitly, or through a deck reserved for it. Its pool stays
// reserved: an ordinary game deals from the ordinary pool and nothing else.
func TestCreateGameRefusesDecklistReaderAndKeepsItsPoolReserved(t *testing.T) {
	enableOpen(t)
	r, gate := freshGameLock(t)
	create := openConfig().createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)

	for i := 0; i < 2; i++ {
		_, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: testOpenPolicy})
		if err == nil || !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Fatalf("explicit net-open pick: error %v, want the decklist refusal", err)
		}
		_, err = create(httpapi.CreateGameOptions{Format: host.FormatConstructed, HumanDeck: "a"})
		if err == nil || !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Fatalf("reserved-deck pick: error %v, want the decklist refusal", err)
		}
	}
	if n := len(r.Tables()); n != 0 {
		t.Fatalf("refused requests created %d table(s)", n)
	}
	for i := 0; i < 6; i++ {
		resp, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: host.BotPolicy})
		if err != nil {
			t.Fatalf("ordinary game %d: %v", i, err)
		}
		for _, d := range waitSeatDecks(t, r, resp.Table) {
			if d != "c" && d != "d" {
				t.Fatalf("an ordinary game dealt %q, a deck reserved for net-open", d)
			}
		}
	}
}

func TestCreateGameRefusesDecksOutsideThePolicysPool(t *testing.T) {
	enableOpen(t)
	r, gate := freshGameLock(t)
	create := openConfig().createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)
	for _, c := range []struct {
		name string
		req  httpapi.CreateGameOptions
		want string
	}{
		{"reserved deck, other policy", httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: host.BotPolicy, HumanDeck: "a"}, "reserved for bot policy \"net-open\""},
		{"reserved bot deck, other policy", httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: host.BotPolicy, BotDeck: "b"}, "reserved for bot policy \"net-open\""},
	} {
		if _, err := create(c.req); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one containing %q", c.name, err, c.want)
		}
	}
	if n := len(r.Tables()); n != 0 {
		t.Errorf("refused requests created %d table(s)", n)
	}
}

// The decklist refusal precedes the payment-plan gate on the vs-bot flow,
// so a mispaid net-open request names the decklist rule. The bot-only table's
// own payment-plan refusal is host's TestHostRefusesDecklistReaderWithoutAutoPay.
func TestCreateGameRefusesDecklistReaderBeforeItsPaymentPlans(t *testing.T) {
	enableOpen(t)
	for _, mut := range []func(*config){func(c *config) { c.autoMana = false }, func(c *config) { c.botAutoPayMana = false }} {
		r, gate := freshGameLock(t)
		c := openConfig()
		mut(&c)
		create := c.createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)
		_, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: testOpenPolicy})
		if err == nil || !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Errorf("error %v, want the decklist refusal", err)
		}
		if n := len(r.Tables()); n != 0 {
			t.Errorf("refused request created %d table(s)", n)
		}
	}
}

// waitSeatDecks returns the decks dealt to a table's two seats once its match
// is visible.
func waitSeatDecks(t *testing.T, r *host.Registry, table string) []string {
	t.Helper()
	for i := 0; i < 20000; i++ {
		ms, err := r.Matches(host.TableID(table))
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) == 1 && len(ms[0].Seats) == 2 {
			return []string{ms[0].Seats[0].Deck, ms[0].Seats[1].Deck}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("no match appeared on table %s", table)
	return nil
}

func writeDeck(t *testing.T, dir, id, extra string) {
	t.Helper()
	body := `{"name":"` + id + `","format":"limited","archetype":"test",` + extra +
		`"cards":[{"name":"Forest","count":20},{"name":"Mountain","count":20}]}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// -net-open-decks is loaded and tagged, and refused when it would let net-open play
// something it was not trained on or make a deck id ambiguous.
func TestNetPoolLoading(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	t.Run("no flag, no pool", func(t *testing.T) {
		ids, cat, err := stubPolicy(testOpenPolicy, netPolicy{}).loadPool(reg, nil, nil, nil, nil)
		if ids != nil || cat != nil || err != nil {
			t.Errorf("got %v %v %v", ids, cat, err)
		}
	})
	t.Run("loaded, sorted and tagged", func(t *testing.T) {
		dir := t.TempDir()
		writeDeck(t, dir, "FDN_b", "")
		writeDeck(t, dir, "FDN_a", "")
		// A deck with no archetype gets the label, so the picker shows what it
		// is. A deck that has one keeps it.
		raw, err := os.ReadFile(filepath.Join(dir, "FDN_a.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "FDN_a.json"), []byte(strings.Replace(string(raw), `"archetype":"test",`, "", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		ids, cat, err := stubPolicy(testOpenPolicy, netPolicy{decks: dir}).loadPool(reg, nil, []string{"other"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, []string{"FDN_a", "FDN_b"}) {
			t.Errorf("pool %v", ids)
		}
		if len(cat) != 2 || cat[0].Pool != testOpenPolicy || cat[1].Pool != testOpenPolicy || cat[0].Format != "constructed" ||
			!strings.Contains(cat[0].Archetype, "net-open") || cat[1].Archetype != "test" {
			t.Errorf("catalogue %+v", cat)
		}
	})
	t.Run("an id that shadows -decks", func(t *testing.T) {
		dir := t.TempDir()
		writeDeck(t, dir, "burn", "")
		if _, _, err := stubPolicy(testOpenPolicy, netPolicy{decks: dir}).loadPool(reg, nil, []string{"burn"}, nil, nil); err == nil || !strings.Contains(err.Error(), "same id") {
			t.Errorf("error %v", err)
		}
	})
	t.Run("an empty directory", func(t *testing.T) {
		if _, _, err := stubPolicy(testOpenPolicy, netPolicy{decks: t.TempDir()}).loadPool(reg, nil, nil, nil, nil); err == nil {
			t.Error("an empty pool was accepted")
		}
	})
	t.Run("a commander deck", func(t *testing.T) {
		dir := t.TempDir()
		writeDeck(t, dir, "cmd", `"commander":"Forest",`)
		if _, _, err := stubPolicy(testOpenPolicy, netPolicy{decks: dir}).loadPool(reg, nil, nil, nil, nil); err == nil {
			t.Error("a commander deck was accepted")
		}
	})
	t.Run("the loader resolves pool ids from their own directory", func(t *testing.T) {
		mainDir, azDir := t.TempDir(), t.TempDir()
		writeDeck(t, mainDir, "ordinary", "")
		writeDeck(t, azDir, "FDN_a", "")
		load := reservedLoader(reg, deckLoader(reg, mainDir), azDir, []string{"FDN_a"})
		for _, id := range []string{"ordinary", "FDN_a"} {
			if d, err := load(id); err != nil || len(d.Cards) != 40 {
				t.Errorf("load(%q) = %d cards, %v", id, len(d.Cards), err)
			}
		}
		if _, err := load("absent"); err == nil {
			t.Error("an unknown id loaded")
		}
	})
}

// The web client names no bot policy, so a deck reserved for one used to
// pick it. Operator ruling 2026-10-13: net-open reads opponents' decklists, so
// a pooled deck can no longer seat a human against it — every such request
// is refused, naming the rule.
func TestCreateGameRefusesAPooledDeckPickOfDecklistReader(t *testing.T) {
	enableOpen(t)
	r, gate := freshGameLock(t)
	c := openConfig()
	c.botPolicy = host.BotPolicy
	create := c.createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)

	for _, req := range []httpapi.CreateGameOptions{
		{Format: host.FormatConstructed, HumanDeck: "a"},
		{Format: host.FormatConstructed, BotDeck: "b"},
		{Format: host.FormatConstructed, HumanDeck: "b", BotDeck: "a"},
	} {
		_, err := create(req)
		if err == nil {
			t.Errorf("%+v: the pooled deck seated net-open against a human", req)
		} else if !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Errorf("%+v: error %v, want the decklist refusal", req, err)
		}
	}
	if n := len(r.Tables()); n != 0 {
		t.Errorf("refused requests created %d table(s)", n)
	}
	// No decks named: the server's own default, from the ordinary pool.
	resp, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed})
	if err != nil {
		t.Fatal(err)
	}
	if resp.BotPolicy != host.BotPolicy {
		t.Errorf("no decks named: opponent %q, want the server default %q", resp.BotPolicy, host.BotPolicy)
	}
	// A pooled deck beside an ordinary one is refused too — now by the
	// decklist rule the pick would have selected net-open for.
	if _, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, HumanDeck: "a", BotDeck: "c"}); err == nil ||
		!strings.Contains(err.Error(), "reads opponents' decklists") {
		t.Errorf("mixed pooled and ordinary decks: error %v, want the decklist refusal", err)
	}
}

func TestDecklistReaderDemoTable(t *testing.T) {
	pool := []string{"FDN_a", "FDN_b"}
	c := config{tables: 4, seed: 10, perpetual: true, autoMana: true, botAutoPayMana: true, mulligans: 2}
	a := stubPolicy(testOpenPolicy, netPolicy{net: "n", demoTable: true})
	got := a.table(c, "t5", 14, pool, view.Public)
	if got == nil {
		t.Fatal("no demo table")
	}
	if got.ID != "t5" || got.Seats != 2 || got.BotPolicy != testOpenPolicy || got.Format != host.FormatConstructed ||
		!reflect.DeepEqual(got.Decks, pool) || len(got.Humans) != 0 || !got.Perpetual || !got.AutoMana || !got.BotAutoPayMana ||
		!got.AllowDecklistBots {
		t.Errorf("demo table %+v", got)
	}
	for name, mut := range map[string]func(*netPolicy){
		"switched off": func(a *netPolicy) { a.demoTable = false },
		"no network":   func(a *netPolicy) { a.net = "" },
	} {
		off := a
		mut(&off)
		if off.table(c, "t5", 14, pool, view.Public) != nil {
			t.Errorf("%s: a demo table was still built", name)
		}
	}
	if a.table(c, "t5", 14, nil, view.Public) != nil {
		t.Error("an empty pool built a demo table")
	}
}

// A decklist reader's -belief reaches its Enable: nil without the flag, every
// deck file, or the train split a -belief-pool names; a directory with none is
// refused.
func TestOpenNetBeliefLoading(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	dir := t.TempDir()
	writeDeck(t, dir, "FDN_a", "")
	writeDeck(t, dir, "FDN_b", "")
	path, err := writeStubNet(dir, "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	var got *searchprobe.Belief
	bot := NetBot{ReadsDecklists: true, Enable: func(_ string, b *searchprobe.Belief) error { calls, got = calls+1, b; return nil }}
	enable := func(belief, pool string) error {
		c := config{allowDecklistBots: true, autoMana: true, botAutoPayMana: true,
			nets: []netPolicy{{policy: "net-belief", bot: bot, net: path, decks: dir, belief: belief, beliefPool: pool}}}
		return c.enableNetBots(reg)
	}
	if err := enable("", ""); err != nil || calls != 1 || got != nil {
		t.Fatalf("no flag: calls %d belief %v, %v", calls, got, err)
	}
	if err := enable(dir, ""); err != nil || got == nil {
		t.Fatalf("-belief did not reach Enable: belief %v, %v", got, err)
	}
	pool := filepath.Join(t.TempDir(), "pool.tsv")
	if err := os.WriteFile(pool, []byte("name\tsplit\tcolors\nFDN_a\ttrain\tRG\nFDN_b\teval\tRG\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enable(dir, pool); err != nil {
		t.Fatal(err)
	}
	if err := enable(t.TempDir(), ""); err == nil {
		t.Fatal("an empty belief directory was accepted")
	}
}

// A closed-list policy's flags: the network needs its decks and its belief
// (closed lists are the point), never -allow-decklist-bots.
func TestClosedNetFlagsValidate(t *testing.T) {
	for _, policy := range closedPolicies {
		p := "-" + policy
		base, full := config{autoMana: true, botAutoPayMana: true}, netPolicy{net: "n.dzgw", decks: "d", belief: "b"}
		if err := withNet(base, policy, full).checkNetFlags(); err != nil {
			t.Fatalf("%s: a complete configuration without -allow-decklist-bots: %v", policy, err)
		}
		cases := []struct {
			name string
			mut  func(*netPolicy, *config)
			want string
		}{
			{"decks without a network", func(a *netPolicy, _ *config) { a.net = "" }, "need " + p + "-net"},
			{"belief without a network", func(a *netPolicy, _ *config) { a.net, a.decks = "", "" }, "need " + p + "-net"},
			{"network without decks", func(a *netPolicy, _ *config) { a.decks = "" }, "needs " + p + "-decks"},
			{"network without a belief", func(a *netPolicy, _ *config) { a.belief = "" }, "needs " + p + "-belief"},
			{"belief pool without a belief", func(a *netPolicy, _ *config) { a.belief, a.beliefPool = "", "pool.tsv" }, p + "-belief-pool needs " + p + "-belief"},
			{"no -auto-mana", func(_ *netPolicy, c *config) { c.autoMana = false }, "-auto-mana"},
			{"no -bot-auto-mana", func(_ *netPolicy, c *config) { c.botAutoPayMana = false }, "-bot-auto-mana"},
		}
		for _, tc := range cases {
			cfg, a := base, full
			tc.mut(&a, &cfg)
			if err := withNet(cfg, policy, a).checkNetFlags(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s %s: error %v, want one containing %q", policy, tc.name, err, tc.want)
			}
		}
	}
}

// The flags are registered under each policy's own prefix, the demo table on
// by default.
func TestNetFlagsRegister(t *testing.T) {
	fs, c := serveFlags()
	if err := fs.Parse([]string{"-net-closed-b-net", "b.net", "-net-closed-b-decks", "db", "-net-closed-b-belief", "bb", "-net-closed-b-belief-pool", "pb",
		"-net-closed-a-net", "a.net", "-net-closed-a-demo-table=false"}); err != nil {
		t.Fatal(err)
	}
	b, a := netOf(*c, testClosedB), netOf(*c, testClosedA)
	if b.net != "b.net" || b.decks != "db" || b.belief != "bb" || b.beliefPool != "pb" || !b.demoTable {
		t.Errorf("%s flags %+v", testClosedB, b)
	}
	if a.net != "a.net" || a.decks != "" || a.demoTable {
		t.Errorf("%s flags %+v", testClosedA, a)
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

// A deck directory shared by net-open, net-closed-a and net-closed-b is one pool reserved for
// all three: catalogued once, picked for a closed-list policy by a request
// that names no policy (net-open cannot seat a human), still refused for an
// explicit net-open pick.
func TestReservedPoliciesShareADeckDirectory(t *testing.T) {
	enableOpen(t)
	enableClosed(t, testClosedA)
	enableClosed(t, testClosedB)
	reg := testutil.CorpusRegistry(t)
	dir := t.TempDir()
	writeDeck(t, dir, "FDN_a", "")
	writeDeck(t, dir, "FDN_b", "")
	c := withNet(withNet(withNet(config{}, testOpenPolicy, netPolicy{decks: dir}), testClosedA, netPolicy{decks: dir + string(filepath.Separator)}), testClosedB, netPolicy{decks: dir})
	poolOpen, cat, err := netOf(c, testOpenPolicy).loadPool(reg, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	prior := []reservedDir{{policy: testOpenPolicy, dir: dir, pool: poolOpen}}
	poolA, cat4, err := netOf(c, testClosedA).loadPool(reg, nil, nil, prior, cat)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(poolOpen, poolA) || len(cat4) != 2 {
		t.Fatalf("pools %v / %v, catalogue %d: want one shared pool catalogued once", poolOpen, poolA, len(cat4))
	}
	if cat[0].Pool != testOpenPolicy {
		t.Fatalf("loadPool wrote through to its catalogue argument: pool %q", cat[0].Pool)
	}
	for _, d := range cat4 {
		if d.Pool != "net-open,net-closed-a" || d.Archetype != "test" {
			t.Errorf("deck %s pool %q archetype %q after net-closed-a", d.ID, d.Pool, d.Archetype)
		}
	}
	prior = append(prior, reservedDir{policy: testClosedA, dir: netOf(c, testClosedA).decks, pool: poolA})
	poolB, cat5, err := netOf(c, testClosedB).loadPool(reg, nil, nil, prior, cat4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(poolOpen, poolB) || len(cat5) != 2 {
		t.Fatalf("pools %v / %v, catalogue %d: want one shared pool catalogued once", poolOpen, poolB, len(cat5))
	}
	for _, d := range cat5 {
		if d.Pool != "net-open,net-closed-a,net-closed-b" || d.Archetype != "test" {
			t.Errorf("deck %s pool %q archetype %q after net-closed-b", d.ID, d.Pool, d.Archetype)
		}
		for _, p := range []string{testOpenPolicy, testClosedA, testClosedB} {
			if !httpapi.PoolHas(d.Pool, p) {
				t.Errorf("deck %s pool %q lacks %s", d.ID, d.Pool, p)
			}
		}
	}
	c.policyPools = map[string][]string{testOpenPolicy: poolOpen, testClosedA: poolA, testClosedB: poolB}
	if got := c.reservedPick("FDN_a"); got != testClosedA {
		t.Errorf("a shared deck picks %q, want net-closed-a (the first owner that may play a human)", got)
	}
	if got := c.reservedFor("FDN_a"); got != "net-closed-a, net-closed-b, net-open" {
		t.Errorf("reservedFor %q", got)
	}
	// Two directories that both hold an id are ambiguous: refused, naming
	// the earlier pool's flag.
	other := t.TempDir()
	writeDeck(t, other, "FDN_a", "")
	if _, _, err := stubPolicy(testClosedA, netPolicy{decks: other}).loadPool(reg, nil, nil, prior[:1], cat); err == nil || !strings.Contains(err.Error(), "-net-open-decks") {
		t.Errorf("net-closed-a: an id in both directories: error %v", err)
	}
	if _, _, err := stubPolicy(testClosedB, netPolicy{decks: other}).loadPool(reg, nil, nil, prior[1:], cat4); err == nil ||
		!strings.Contains(err.Error(), "-net-closed-b-decks") || !strings.Contains(err.Error(), "-net-closed-a-decks") {
		t.Errorf("net-closed-b: an id in net-closed-a's directory and its own: error %v", err)
	}
	// A closed-list pool alone: its decks are catalogued for it only.
	solo, catSolo, err := stubPolicy(testClosedB, netPolicy{decks: other}).loadPool(reg, nil, nil, nil, nil)
	if err != nil || len(solo) != 1 || len(catSolo) != 1 || catSolo[0].Pool != testClosedB {
		t.Errorf("net-closed-b alone: pool %v catalogue %+v, %v", solo, catSolo, err)
	}
	// A deck with the default archetype names every owner; one with its own
	// ("test", above) keeps it.
	def := []httpapi.DeckInfo{{ID: "FDN_a", Pool: testOpenPolicy, Archetype: "plays vs net-open"}}
	_, def4, err := netOf(c, testClosedA).loadPool(reg, nil, nil, prior[:1], def)
	if err != nil || def4[0].Archetype != "plays vs net-open or net-closed-a" {
		t.Errorf("default archetype after net-closed-a: %+v, %v", def4, err)
	}
	_, def5, err := netOf(c, testClosedB).loadPool(reg, nil, nil, prior, def4[:1])
	if err != nil || def5[0].Archetype != "plays vs net-open, net-closed-a or net-closed-b" {
		t.Errorf("default archetype after net-closed-b: %+v, %v", def5, err)
	}
	// The loader resolves a shared id from the shared directory.
	load := deckLoader(reg, t.TempDir())
	for _, a := range c.nets {
		load = reservedLoader(reg, load, a.decks, c.policyPools[a.policy])
	}
	if d, err := load("FDN_b"); err != nil || len(d.Cards) != 40 {
		t.Errorf("load(FDN_b) = %d cards, %v", len(d.Cards), err)
	}
	// Every policy has its reserved pool (draft events still leave out the
	// decklist reader: eventPolicies).
	var reserved []string
	for p := range c.policyPools {
		reserved = append(reserved, p)
	}
	sort.Strings(reserved)
	if got := reserved; !reflect.DeepEqual(got, []string{testClosedA, testClosedB, testOpenPolicy}) {
		t.Errorf("reserved policies %v", got)
	}
}

// Each closed-list policy's demo table is a bot-only table that needs no
// decklist opt-in.
func TestClosedNetDemoTables(t *testing.T) {
	for _, policy := range closedPolicies {
		c := config{autoMana: true, botAutoPayMana: true}
		a := stubPolicy(policy, netPolicy{net: "n", demoTable: true})
		tc := a.table(c, "t3", 7, []string{"a", "b"}, view.Public)
		if tc == nil || tc.BotPolicy != policy || tc.AllowDecklistBots || len(tc.Humans) != 0 || tc.ID != "t3" || tc.Name != policy+" vs "+policy {
			t.Fatalf("%s demo table %+v", policy, tc)
		}
		a.demoTable = false
		if a.table(c, "t3", 7, []string{"a"}, view.Public) != nil {
			t.Fatalf("-%s-demo-table=false still built the table", policy)
		}
	}
}

// withRegisteredNetBot runs with policy registered through RegisterNetBot,
// restoring the linked set afterwards. The bots registry entry an Enable adds
// stays (it is process-wide), so each test uses its own policy name.
func withRegisteredNetBot(t *testing.T, policy string, nb NetBot) {
	t.Helper()
	netBotsMu.Lock()
	saved := append([]registeredNetBot(nil), netBots...)
	netBotsMu.Unlock()
	t.Cleanup(func() {
		netBotsMu.Lock()
		netBots = saved
		netBotsMu.Unlock()
	})
	RegisterNetBot(policy, nb)
}

// A registered policy's five flags exist and parse, it follows the earlier
// registrations, its flags are checked, and with them set it is registered
// closed over its belief and offered to draft events.
func TestRegisterNetBotServesAPolicy(t *testing.T) {
	const policy = "net-served"
	sb := &stubNetBot{info: stubInfo(policy, true)}
	withRegisteredNetBot(t, policy, NetBot{NetHelp: "its .dzgw network (a test net)", Enable: sb.enable})
	dir := t.TempDir()
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
	fs, c := serveFlags()
	fs.SetOutput(nil)
	args := []string{"-" + policy + "-net", path, "-" + policy + "-decks", dir, "-" + policy + "-belief", belief, "-" + policy + "-demo-table=false"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	if f := fs.Lookup(policy + "-net"); f == nil || !strings.Contains(f.Usage, "a test net") {
		t.Fatalf("-%s-net flag %+v: want the registered help", policy, f)
	}
	var order []string
	for _, a := range c.nets {
		order = append(order, a.policy)
	}
	if want := []string{testOpenPolicy, testClosedA, testClosedB, policy}; !reflect.DeepEqual(order, want) {
		t.Fatalf("policy order %v, want %v", order, want)
	}
	a := netOf(*c, policy)
	if a.net != path || a.decks != dir || a.belief != belief || a.demoTable {
		t.Fatalf("parsed flags %+v", a)
	}
	bad := *c
	bad.nets = []netPolicy{{policy: policy, bot: a.bot, net: path, decks: dir}}
	bad.autoMana, bad.botAutoPayMana = true, true
	if err := bad.checkNetFlags(); err == nil || !strings.Contains(err.Error(), "-"+policy+"-net needs -"+policy+"-belief") {
		t.Errorf("no belief: error %v", err)
	}
	c.autoMana, c.botAutoPayMana = true, true
	if err := c.enableNetBots(testutil.CorpusRegistry(t)); err != nil {
		t.Fatal(err)
	}
	e, ok := bots.Lookup(policy)
	if !ok || !e.ClosedLists || bots.ReadsDecklists(policy) {
		t.Fatalf("after enable: registered %v, closed %v", ok, e.ClosedLists)
	}
	c.botPolicies = []string{bots.Default, policy}
	if got := c.eventPolicies(); !reflect.DeepEqual(got, c.botPolicies) {
		t.Errorf("event policies %v, want %v", got, c.botPolicies)
	}
}

func TestRegisterNetBotRefusals(t *testing.T) {
	sb := &stubNetBot{info: stubInfo("x", true)}
	withRegisteredNetBot(t, "net-dup", NetBot{Enable: sb.enable})
	refused := func(name string, policy string, enable func(string, *searchprobe.Belief) error) {
		defer func() {
			if recover() == nil {
				t.Errorf("RegisterNetBot accepted %s", name)
			}
		}()
		RegisterNetBot(policy, NetBot{Enable: enable})
	}
	for _, policy := range []string{"net-dup", "", "Net-C6", "net c6", "-net", "net-"} {
		refused(policy, policy, sb.enable)
	}
	refused("a nil Enable", "net-nil", nil)
}

// Extensions.Flags puts a binary's own flags on gorged's command line: the
// flag set Main and Serve parse, and ServeFlagSetFor's, carry them, and
// gorge's own ServeFlagSet does not.
func TestExtensionFlags(t *testing.T) {
	var got string
	ext := Extensions{Flags: func(fs *flag.FlagSet) { fs.StringVar(&got, "ext-test-flag", "def", "an extension's flag") }}
	fs, c := flagsFor(ext)
	fs.SetOutput(nil)
	if err := fs.Parse([]string{"-ext-test-flag", "v", "-events"}); err != nil {
		t.Fatal(err)
	}
	if got != "v" || !c.events || c.ext.Flags == nil {
		t.Fatalf("ext flag %q, events %v, ext carried %v", got, c.events, c.ext.Flags != nil)
	}
	if err := ServeFlagSetFor(ext).Parse([]string{"-ext-test-flag", "w"}); err != nil || got != "w" {
		t.Fatalf("ServeFlagSetFor: %v (value %q)", err, got)
	}
	if err := ServeFlagSet().Parse([]string{"-ext-test-flag", "w"}); err == nil {
		t.Fatal("gorge's own flag set accepted an extension's flag")
	}
}
