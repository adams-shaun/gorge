package gorged

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// routedServer starts a gorge server with -route-pods-dirac over edh, dealing
// only foundations-wretched-ranks, the one Commander deck edh plays, and
// returns its base URL, an edh engine and its -dir. It skips when the edh
// tables are not built (make dirac-tables).
func routedServer(t *testing.T) (string, *adapter.Engine, string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	edh, err := adapter.New([]string{"edh"}, reg, dirac.SurfaceFull)
	if err != nil {
		t.Skipf("edh tables are not built (make dirac-tables): %v", err)
	}
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testkit", "testutil", "decks", "foundations-wretched-ranks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundations-wretched-ranks.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	c := config{diracCheckSets: "edh", routePodsDirac: true, seats: 4, formatsRaw: "commander", decks: dir, decksGiven: true,
		vsbot: true, botPolicy: "bot", autoMana: true, botAutoPayMana: true, dir: state}
	base, cancel, _ := startServe(t, c)
	t.Cleanup(cancel)
	return base, edh, state
}

// builtEngine is the engine tables.json under dir records for table id: the
// engine the host actually built it on, "" for gorge (the key is omitted).
func builtEngine(t *testing.T, dir, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "tables.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Tables []struct {
			Config struct {
				ID     string `json:"id"`
				Engine string `json:"engine"`
			} `json:"config"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for _, tb := range f.Tables {
		if tb.Config.ID == id {
			return tb.Config.Engine
		}
	}
	t.Fatalf("tables.json holds no table %s: %s", id, raw)
	return ""
}

// commanderText is f as a submitted text deck: its commander, then each other card once per copy.
func commanderText(f deck.File) string {
	cmdr := f.CommanderNames()[0]
	var lines []string
	for _, e := range f.Cards {
		if e.Name != cmdr {
			for range e.Count {
				lines = append(lines, "1 "+e.Name)
			}
		}
	}
	return "Commander\n1 " + cmdr + "\n\n" + strings.Join(lines, "\n") + "\n"
}

// wretchedRanks is the deck file every seat of routedServer plays.
func wretchedRanks(t *testing.T) deck.File {
	t.Helper()
	f, _, err := deck.Load(testutil.CorpusRegistry(t), filepath.Join("..", "..", "testkit", "testutil", "decks", "foundations-wretched-ranks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func postRouted(t *testing.T, base string, body map[string]any) httpapi.CreateGameResponse {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(base+"/api/games", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/games: %d %s", resp.StatusCode, out)
	}
	var g httpapi.CreateGameResponse
	if err := json.Unmarshal(out, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// TestCreatePodRoutedToDirac: on a gorge server with -route-pods-dirac, a
// four-seat commander pod of decks edh plays runs on dirac, and its 4-seat
// view comes up.
func TestCreatePodRoutedToDirac(t *testing.T) {
	base, _, state := routedServer(t)
	g := postRouted(t, base, map[string]any{"format": "commander", "seats": 4, "humans": 1, "bot_policy": "bot"})
	if g.Engine != adapter.Name || g.EngineReason != "" {
		t.Fatalf("engine %q reason %q, want dirac and no reason", g.Engine, g.EngineReason)
	}
	if got := builtEngine(t, state, g.Table); got != adapter.Name {
		t.Fatalf("the response says dirac but table %s was built on %q", g.Table, got)
	}
	// GET /api/tables lists no on-demand table (Registry.LobbyTables), so the
	// pod's own view, behind its seat token, is what shows it came up.
	req, _ := http.NewRequest(http.MethodGet, base+"/api/tables/"+g.Table+"/matches/1/view?seat=0", nil)
	req.Header.Set("Authorization", "Bearer "+g.Token)
	for deadline := time.Now().Add(testutil.WaitBudget); ; time.Sleep(time.Second / 50) {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Players []json.RawMessage `json:"players"`
		}
		err = json.NewDecoder(resp.Body).Decode(&v)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK && err == nil && len(v.Players) == 4 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the routed pod %s shows no 4-seat view: %d %v", g.Table, resp.StatusCode, err)
		}
	}
}

// TestCreatePodFallsBackToGorge: the same server seats a pod whose seat-0
// text deck edh refuses on gorge, naming seat 0's refusal.
func TestCreatePodFallsBackToGorge(t *testing.T) {
	base, edh, state := routedServer(t)
	reg := testutil.CorpusRegistry(t)
	dir := filepath.Join("..", "..", "testkit", "testutil", "decks")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	text, picked := "", ""
	for _, path := range files {
		f, cs, err := deck.Load(reg, path)
		if err != nil || len(f.CommanderNames()) != 1 || edh.CheckDeck(cs) == nil {
			continue
		}
		text, picked = commanderText(f), path
		break
	}
	if text == "" {
		t.Skipf("no single-commander deck under %s is refused by edh", dir)
	}
	g := postRouted(t, base, map[string]any{"format": "commander", "seats": 4, "humans": 1, "bot_policy": "bot",
		"decks": []map[string]any{{"seat": 0, "text": text}}})
	if g.Engine != gorgegame.Name || !strings.HasPrefix(g.EngineReason, "seat 0 deck: ") ||
		strings.Contains(g.EngineReason, "was not checked on dirac at startup") {
		t.Fatalf("%s: engine %q reason %q, want gorge and seat 0's CheckDeck refusal", picked, g.Engine, g.EngineReason)
	}
	if got := builtEngine(t, state, g.Table); got != "" {
		t.Fatalf("table %s fell back to gorge but was built on %q", g.Table, got)
	}
}

// TestCreatePodSubmittedDeckRoutedToDirac: a submitted seat-0 deck edh plays
// is checked on its seated cards, so the pod still runs on dirac.
func TestCreatePodSubmittedDeckRoutedToDirac(t *testing.T) {
	base, _, state := routedServer(t)
	g := postRouted(t, base, map[string]any{"format": "commander", "seats": 4, "humans": 1, "bot_policy": "bot",
		"decks": []map[string]any{{"seat": 0, "text": commanderText(wretchedRanks(t))}}})
	if g.Engine != adapter.Name || g.EngineReason != "" {
		t.Fatalf("engine %q reason %q, want dirac and no reason", g.Engine, g.EngineReason)
	}
	if got := builtEngine(t, state, g.Table); got != adapter.Name {
		t.Fatalf("table %s was built on %q, want dirac", g.Table, got)
	}
}

// TestCreatePodDiracRefusalFallsBack: a submitted deck with a sideboard passes
// CheckDeck, but dirac's Accepts refuses the pod; the table is retried once on
// gorge and the refusal becomes the reason.
func TestCreatePodDiracRefusalFallsBack(t *testing.T) {
	base, _, state := routedServer(t)
	f := wretchedRanks(t)
	card := func(n string) mb.DeckCard { return mb.DeckCard{Identity: mb.DeckCardIdentity{Name: n}} }
	d := mb.Deck{Name: "Sideboarded", Format: "commander", Commanders: []mb.DeckCard{card(f.CommanderNames()[0])},
		Sideboard: []mb.DeckCard{card("Swamp")}}
	for _, e := range f.Cards {
		for range e.Count {
			d.Cards = append(d.Cards, card(e.Name))
		}
	}
	g := postRouted(t, base, map[string]any{"format": "commander", "seats": 4, "humans": 1, "bot_policy": "bot",
		"decks": []map[string]any{{"seat": 0, "deck": d}}})
	if g.Engine != gorgegame.Name || !strings.HasPrefix(g.EngineReason, "dirac refused the pod: ") {
		t.Fatalf("engine %q reason %q, want gorge and dirac's refusal", g.Engine, g.EngineReason)
	}
	if got := builtEngine(t, state, g.Table); got != "" {
		t.Fatalf("table %s was built on %q, want gorge", g.Table, got)
	}
}
