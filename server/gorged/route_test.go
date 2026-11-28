package gorged

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/search"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/server/host"
)

// sentinelChecker fails a deck that holds the sentinel card.
type sentinelChecker struct{ sentinel *cards.Card }

func (s sentinelChecker) CheckDeck(d []*cards.Card) error {
	for _, c := range d {
		if c == s.sentinel {
			return errors.New("sentinel unsupported")
		}
	}
	return nil
}

func TestRoutePodConditions(t *testing.T) {
	sentinel, plain := &cards.Card{Path: "sentinel"}, &cards.Card{Path: "plain"}
	check := sentinelChecker{sentinel}
	if e, ok := bots.Lookup(search.Policy); !ok || len(seatablePolicies([]string{search.Policy})) != 0 || !e.Search {
		t.Fatalf("precondition: %q must be registered and not seatable on dirac", search.Policy)
	}
	on := config{routePodsDirac: true, autoMana: true, botAutoPayMana: true, diracPool: map[string]error{"pool-a": nil}}
	decks := func() []routeDeck {
		return []routeDeck{{PoolID: "pool-a"}, {Cards: []*cards.Card{plain}}, {PoolID: "pool-a"}, {Cards: []*cards.Card{plain}}}
	}
	type args struct {
		c      config
		format host.Format
		seats  int
		policy string
		paired string
		allow  bool
		decks  []routeDeck
	}
	base := func() args { return args{on, host.FormatCommander, 4, "bot", "", false, decks()} }
	for _, tc := range []struct {
		name         string
		set          func(*args)
		engine, want string
	}{
		{"all pass", func(*args) {}, adapter.Name, ""},
		{"two seats", func(a *args) { a.seats = 2 }, gorgegame.Name, "pods route to dirac only at 4 seats in commander"},
		{"constructed", func(a *args) { a.format = host.FormatConstructed }, gorgegame.Name, "pods route to dirac only at 4 seats in commander"},
		{"search policy", func(a *args) { a.policy = search.Policy }, gorgegame.Name, `bot policy "search" cannot be seated on dirac`},
		{"paired bot", func(a *args) { a.paired = "x" }, gorgegame.Name, "a paired bot needs gorge"},
		{"allow paired", func(a *args) { a.allow = true }, gorgegame.Name, "a paired bot needs gorge"},
		{"manual mana", func(a *args) { a.c.autoMana = false }, gorgegame.Name, "dirac has no manual mana"},
		{"bot manual mana", func(a *args) { a.c.botAutoPayMana = false }, gorgegame.Name, "dirac has no manual mana"},
		{"seat 2 sentinel", func(a *args) { a.decks[2] = routeDeck{Cards: []*cards.Card{plain, sentinel}} }, gorgegame.Name, "seat 2 deck: sentinel unsupported"},
		{"pool deck refused", func(a *args) {
			a.c.diracPool = map[string]error{"pool-a": nil, "pool-b": errors.New("no")}
			a.decks[1] = routeDeck{PoolID: "pool-b"}
		}, gorgegame.Name, "seat 1 deck: no"},
		{"flag off", func(a *args) { a.c.routePodsDirac = false; a.seats = 2 }, gorgegame.Name, ""},
	} {
		a := base()
		tc.set(&a)
		got := a.c.routePod(check, a.format, a.seats, a.policy, a.paired, a.allow, a.decks)
		if got.Engine != tc.engine || got.Reason != tc.want {
			t.Errorf("%s: got %+v, want engine %q reason %q", tc.name, got, tc.engine, tc.want)
		}
		if strings.HasPrefix(tc.name, "seat 2") && !strings.HasPrefix(got.Reason, "seat 2 deck: ") {
			t.Errorf("%s: reason %q does not start seat 2 deck: ", tc.name, got.Reason)
		}
	}
}

func TestRoutePodsDiracFlagNeedsSets(t *testing.T) {
	c := config{routePodsDirac: true}
	if err := c.checkRouteFlags(); err == nil || err.Error() != "-route-pods-dirac needs -dirac-check-sets: the sets a routed pod plays on" {
		t.Errorf("no sets: %v", err)
	}
	c = config{routePodsDirac: true, diracCheckSets: "edh", engine: adapter.Name}
	if err := c.checkRouteFlags(); err == nil || err.Error() != "-route-pods-dirac routes a gorge server's pods; -engine dirac already plays every table on dirac" {
		t.Errorf("engine dirac: %v", err)
	}
	c = config{routePodsDirac: true, diracCheckSets: "edh"}
	if err := c.checkRouteFlags(); err != nil {
		t.Errorf("a valid route config: %v", err)
	}
	if err := (config{engine: adapter.Name}).checkRouteFlags(); err != nil {
		t.Errorf("flag off: %v", err)
	}
}

// TestServeChecksRouteFlags: serve refuses -route-pods-dirac without
// -dirac-check-sets before it opens the corpus.
func TestServeChecksRouteFlags(t *testing.T) {
	err := serve(context.Background(), config{engine: gorgegame.Name, routePodsDirac: true}, nil)
	if err == nil || err.Error() != "-route-pods-dirac needs -dirac-check-sets: the sets a routed pod plays on" {
		t.Fatalf("serve: %v", err)
	}
}

// TestCreateTwoSeatNamesGorge: a two-seat create on a server without
// -route-pods-dirac answers engine gorge and no reason.
func TestCreateTwoSeatNamesGorge(t *testing.T) {
	base, cancel, _ := startServe(t, config{vsbot: true})
	defer cancel()
	status, body := postGame(t, base, map[string]string{"format": "constructed", "bot_policy": "bot"})
	if status != http.StatusOK {
		t.Fatalf("POST /api/games: %d %s", status, body)
	}
	var g map[string]any
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatal(err)
	}
	if _, has := g["engine_reason"]; g["engine"] != gorgegame.Name || has {
		t.Fatalf("engine %v reason %v, want gorge and no reason key", g["engine"], g["engine_reason"])
	}
}
