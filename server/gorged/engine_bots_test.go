package gorged

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/forgeheuristic"
	"github.com/adams-shaun/gorge/bots/sbtactical"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestRotationSkipsARefusedPair(t *testing.T) {
	refuse := func(pairs ...string) func(a, b string) error {
		return func(a, b string) error {
			if slices.Contains(pairs, a+b) {
				return errors.New("refused")
			}
			return nil
		}
	}
	for _, tc := range []struct {
		name    string
		accepts func(a, b string) error
		want    []string
	}{
		{"all legal", refuse(), []string{"a", "b", "c"}},
		{"a pair in the middle", refuse("ab"), []string{"a", "c"}},
		{"the pair that closes it", refuse("ca"), []string{"a", "b"}},
		{"one left plays itself", refuse("ab", "ac"), []string{"a"}},
	} {
		got, err := rotationDecks([]string{"a", "b", "c"}, tc.accepts)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v, %v", tc.name, got, err)
		}
	}
	if _, err := rotationDecks([]string{"a", "b"}, refuse("ab", "aa")); err == nil || !strings.Contains(err.Error(), "no legal pair") {
		t.Errorf("nothing legal: %v", err)
	}
}

// botGames plays pairs of bench decks with policy in both seats and fails on any
// game that did not finish or that a bot needed the host's refusal ladder in.
func botGames(t *testing.T, policy string, pairs int) (rootRefusals int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, err := adapter.New([]string{"fdn"}, reg, dirac.SurfaceFull)
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	r, err := host.New(host.Options{
		Dir: dir,
		LoadDeck: func(name string) (host.Deck, error) {
			_, d, err := deck.Load(reg, filepath.Join(benchDecks, name+".json"))
			if err != nil {
				return host.Deck{}, err
			}
			return host.Deck{Name: name, Cards: d}, nil
		},
		Tokens:  reg.Tokens,
		Engines: map[string]game.Engine{adapter.Name: e},
		BotDeps: bots.Deps{Cards: reg},
		Sleep:   func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	entries, err := os.ReadDir(benchDecks)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < pairs; i++ {
		id := host.TableID(fmt.Sprintf("p%d", i))
		decks := []string{strings.TrimSuffix(entries[i].Name(), ".json"), strings.TrimSuffix(entries[i+1].Name(), ".json")}
		if err := r.AddTable(host.TableConfig{ID: id, Name: string(id), Seats: 2, Decks: decks, Seed: uint64(i + 1), Spectator: view.Public,
			BotPolicy: policy, AutoMana: true, BotAutoPayMana: true, AllowDecklistBots: true, Engine: adapter.Name}); err != nil {
			t.Fatal(err)
		}
		if err := r.Start(id); err != nil {
			t.Fatal(err)
		}
		r.Wait(id)
		ms, err := r.Matches(id)
		if err != nil {
			t.Fatal(err)
		}
		var sc struct {
			Reason       string `json:"reason"`
			Refusals     int    `json:"refusals"`
			Fallbacks    int    `json:"fallbacks"`
			RootRefusals int    `json:"root_refusals"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, string(id), "1.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &sc); err != nil {
			t.Fatal(err)
		}
		if len(ms) != 1 || ms[0].State != protocol.MatchFinished {
			t.Errorf("%s %v: %s after %d turns: %s", policy, decks, ms[0].State, ms[0].Turns, sc.Reason)
		}
		if sc.Refusals != 0 || sc.Fallbacks != 0 {
			t.Errorf("%s %v: %d refusals, %d fallbacks, %s", policy, decks, sc.Refusals, sc.Fallbacks, sc.Reason)
		}
		rootRefusals += sc.RootRefusals
	}
	return rootRefusals
}

// TestBotsPlayDiracGames seats each policy dirac can serve on both sides of 40 pairs.
// sb-tactical sits only at a bot table, whose decklist opt-in its Env declaration asks
// for though no root reaches it here.
func TestBotsPlayDiracGames(t *testing.T) {
	for _, policy := range []string{host.BotPolicy, "lethal-pressure", forgeheuristic.Policy, forgeheuristic.HoldPolicy} {
		t.Run(policy, func(t *testing.T) { botGames(t, policy, 40) })
	}
	t.Run(sbtactical.Policy, func(t *testing.T) {
		if refused := botGames(t, sbtactical.Policy, 40); refused == 0 {
			t.Error("sb-tactical asked for no root: the refused planner went unexercised")
		}
	})
}

// TestSearchPoliciesRefusedByName holds the rule to every registered policy.
func TestSearchPoliciesRefusedByName(t *testing.T) {
	testutil.CorpusRegistry(t)
	var offered []string
	for _, e := range bots.Entries() {
		offered = append(offered, e.Name)
	}
	seatable := seatablePolicies(offered)
	for _, e := range bots.Entries() {
		if got := slices.Contains(seatable, e.Name); got == (e.Search || e.ReadsDecklists()) {
			t.Errorf("%s: seatable %v, search %v, reads decklists %v", e.Name, got, e.Search, e.ReadsDecklists())
		}
	}
}

// TestSearchPolicyRefusedOnADiracGame names the policy to the client asking for it.
func TestSearchPolicyRefusedOnADiracGame(t *testing.T) {
	c := diracConfig()
	c.vsbot, c.botPolicy = true, "bot"
	base, cancel, _ := startServe(t, c)
	defer cancel()
	status, body := postGame(t, base, map[string]string{"format": "constructed", "bot_policy": "az-redeal"})
	if status != http.StatusBadRequest || !strings.Contains(string(body), "az-redeal") {
		t.Fatalf("a search policy: %d %s", status, body)
	}
}
