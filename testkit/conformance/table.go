package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

const tableName = "conformance"

// startTable starts c on e as a host table with every seat human, on auto-pay.
// A table takes c's deck identity from Names, so two seats may share a name only
// by sharing a deck.
func startTable(t testing.TB, wire string, e game.Engine, c game.Config) *host.Registry {
	t.Helper()
	if c.StartChooser != nil || c.DisablePaidManaCensus || c.NoObjectCap || c.NameUniverseNames != nil || c.Key != "" {
		t.Fatalf("%s wire: a table cannot express StartChooser, DisablePaidManaCensus, NoObjectCap, NameUniverseNames or Key", wire)
	}
	seats := len(c.Decks)
	decks := make(map[string]host.Deck, seats)
	tableDecks := make([]string, seats)
	humans := make([]int, seats)
	for i, name := range c.Names {
		if prior, ok := decks[name]; ok && !slices.Equal(prior.Cards, c.Decks[i]) {
			t.Fatalf("%s wire: two different decks are named %q", wire, name)
		}
		d := host.Deck{Name: name, Cards: c.Decks[i]}
		if c.Sideboards != nil {
			d.Sideboard = c.Sideboards[i]
		}
		if c.Archetypes != nil {
			d.Archetype = c.Archetypes[i]
		}
		if c.Commanders != nil {
			d.Commanders = c.Commanders[i]
		}
		decks[name] = d
		// Seat i of a table's match k plays Decks[(i+k)%len], and this is match 1.
		tableDecks[(i+1)%seats] = name
		humans[i] = i
	}
	guard := host.DefaultMaxDecisionsPerTurn
	if c.GuardDisabled {
		guard = 0
	}
	r, err := host.New(host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			d, ok := decks[name]
			if !ok {
				return host.Deck{}, fmt.Errorf("no deck %q", name)
			}
			return d, nil
		},
		Tokens:              c.Tokens,
		NameUniverse:        c.NameUniverse,
		Engines:             map[string]game.Engine{e.Name(): e},
		MaxDecisionsPerTurn: guard,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: tableName, Name: tableName, Seats: seats, Decks: tableDecks, Seed: c.Seed,
		PlayerNames: c.PlayerNames, Spectator: view.Omniscient, Humans: humans, AutoMana: true, Mulligans: c.Mulligans,
		WindowDiagnostics: c.WindowDiagnostics, Format: host.Format(c.Format), StartingLife: c.StartingLife, Engine: e.Name()}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(tableName); err != nil {
		t.Fatal(err)
	}
	return r
}

// tableFinished asks the native API whether the table's match has ended. A match
// that ended any way but finishing fails the test.
func tableFinished(t testing.TB, wire string, spectator http.Handler) bool {
	t.Helper()
	status, body := nativeDo(spectator, http.MethodGet, "/api/tables/"+tableName+"/matches", nil)
	if status != http.StatusOK {
		t.Fatalf("%s wire: listing matches: status %d: %s", wire, status, body)
	}
	var infos []protocol.MatchInfo
	if err := json.Unmarshal(body, &infos); err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].State == protocol.MatchLive {
		return false
	}
	if infos[0].State != protocol.MatchFinished {
		t.Fatalf("%s wire: match ended %s, seed %d: %s", wire, infos[0].State, infos[0].Seed, infos[0].Result)
	}
	return true
}
