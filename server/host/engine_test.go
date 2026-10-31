package host_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const scriptName = "script"

// scriptEngine plays one game: each seat is asked one question, then seat 1
// wins.
type scriptEngine struct{ refuse error }

func (scriptEngine) Name() string { return scriptName }

func (e scriptEngine) Accepts(game.Config) error { return e.refuse }

func (scriptEngine) New(c game.Config) (game.Game, error) {
	g := &scriptGame{board: state.NewGame(c.Names), log: events.NewLog(c.Seed)}
	g.ask(0)
	return g, nil
}

func (e scriptEngine) Replay(l *events.Log, c game.Config) (game.Game, error) {
	return e.ReplayTo(l, c, len(l.Intents))
}

func (e scriptEngine) ReplayTo(l *events.Log, c game.Config, n int) (game.Game, error) {
	return e.Walk(l, c, n, func(game.Game, int) error { return nil })
}

func (e scriptEngine) Walk(l *events.Log, c game.Config, n int, visit func(game.Game, int) error) (game.Game, error) {
	g, _ := e.New(c)
	for i, in := range l.Intents[:min(n, len(l.Intents))] {
		if err := visit(g, i); err != nil {
			return g, err
		}
		if err := g.Submit(in); err != nil {
			return g, err
		}
	}
	return g, visit(g, min(n, len(l.Intents)))
}

type scriptGame struct {
	board   *state.Game
	log     *events.Log
	pending *decision.Decision
}

func (g *scriptGame) emit(e events.Event) {
	e = g.log.Append(e)
	events.Apply(g.board, e)
}

func (g *scriptGame) ask(p state.PlayerID) {
	g.emit(events.Event{Kind: events.DecisionAsk, Player: p, Text: "pass?"})
	g.pending = &decision.Decision{Seq: uint64(len(g.log.Events)), Player: p, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}}
}

func (g *scriptGame) Board() *state.Game { return g.board }
func (g *scriptGame) Chars() view.Chars  { return scriptChars{} }
func (g *scriptGame) Log() *events.Log   { return g.log }

func (g *scriptGame) Pending() *decision.Decision { return g.pending }

func (g *scriptGame) Submit(in decision.Intent) error {
	if g.pending == nil {
		return errors.New("the game is over")
	}
	if err := g.pending.Validate(in); err != nil {
		return err
	}
	g.log.Intents = append(g.log.Intents, in)
	if g.pending.Player == 0 {
		g.ask(1)
		return nil
	}
	g.pending = nil
	g.emit(events.Event{Kind: events.GameOver, Player: 1})
	return nil
}

func (g *scriptGame) Fork() game.Game {
	f := &scriptGame{board: g.board.Clone(), log: g.log.Clone()}
	if g.pending != nil {
		d := *g.pending
		f.pending = &d
	}
	return f
}

func (g *scriptGame) Caps() game.Caps { return game.Caps{Engine: scriptName} }

// scriptChars answers for a board with no objects.
type scriptChars struct{}

func (scriptChars) Power(state.ObjID) int32                 { return 0 }
func (scriptChars) Toughness(state.ObjID) int32             { return 0 }
func (scriptChars) Keywords(state.ObjID) []string           { return nil }
func (scriptChars) PendingTriggers() []state.PendingTrigger { return nil }
func (scriptChars) StackOptional(state.ObjID) (bool, state.PlayerID) {
	return false, 0
}
func (scriptChars) AvailableMana(state.PlayerID) state.Mana           { return state.Mana{} }
func (scriptChars) AbilityCosts(state.PlayerID, state.ObjID) []string { return nil }
func (scriptChars) MayLookAtLibraryTop(state.PlayerID) bool           { return false }
func (scriptChars) PotentialActions(state.PlayerID) []decision.PotentialAction {
	return nil
}
func (scriptChars) OwnDeck(state.PlayerID) *deck.Manifest { return nil }

func scriptOptions(dir string, e game.Engine) host.Options {
	o := host.Options{
		Dir:      dir,
		LoadDeck: func(name string) (host.Deck, error) { return host.Deck{Name: name}, nil },
		Sleep:    func(time.Duration, <-chan struct{}) {},
	}
	if e != nil {
		o.Engines = map[string]game.Engine{e.Name(): e}
	}
	return o
}

func scriptTable() host.TableConfig {
	return host.TableConfig{ID: "s", Name: "s", Seats: 2, Decks: []string{"a", "b"}, Seed: 3,
		Spectator: view.Public, Humans: []int{0, 1}, Engine: scriptName}
}

// playScript answers every decision of table s through the HTTP API, then
// returns a client for the finished match.
func playScript(t *testing.T, r *host.Registry) nativeSeamClient {
	t.Helper()
	var seat state.PlayerID
	api := nativeSeamClient{t: t, h: httpapi.NewHandler(r, httpapi.Options{
		Seat: func(*http.Request) (httpapi.SeatClaim, bool) { return httpapi.SeatClaim{Table: "s", Seat: seat}, true },
	})}
	const match = "/api/tables/s/matches/1"
	deadline := time.Now().Add(time.Minute)
	for answered := 0; answered < 2; {
		if time.Now().After(deadline) {
			t.Fatalf("no decision after %d answers", answered)
		}
		seat = state.PlayerID(answered)
		status, body := api.do(http.MethodGet, match+"/pending?seat="+strconv.Itoa(answered), nil)
		if status != http.StatusOK {
			time.Sleep(time.Millisecond)
			continue
		}
		var d decision.Decision
		if err := json.Unmarshal(body, &d); err != nil {
			t.Fatal(err)
		}
		if int(d.Player) != answered {
			t.Fatalf("answer %d: decision for seat %d", answered, d.Player)
		}
		api.get(match + "/view?seat=" + strconv.Itoa(answered))
		in, err := json.Marshal(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}})
		if err != nil {
			t.Fatal(err)
		}
		if status, resp := api.do(http.MethodPost, match+"/intent", in); status != http.StatusNoContent {
			t.Fatalf("answer %d: status %d: %s", answered, status, resp)
		}
		answered++
	}
	r.Wait("s")
	return api
}

func TestStubEngineServesAMatch(t *testing.T) {
	t.Parallel()
	r, err := host.New(scriptOptions("", scriptEngine{}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(scriptTable()); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("s"); err != nil {
		t.Fatal(err)
	}
	api := playScript(t, r)

	ms, err := r.Matches("s")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].State != protocol.MatchFinished || ms[0].Winner == nil || *ms[0].Winner != 1 {
		t.Fatalf("matches %+v", ms)
	}
	if got := r.Tables()[0].Engine; got != scriptName {
		t.Fatalf("table engine %q", got)
	}
	var evs []protocol.EventBody
	if err := json.Unmarshal(api.get("/api/tables/s/matches/1/events?since=0"), &evs); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 || evs[2].Event.Kind != "game_over" {
		t.Fatalf("events %+v", evs)
	}
	var v view.View
	if err := json.Unmarshal(api.get("/api/tables/s/matches/1/view"), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Players) != 2 {
		t.Fatalf("view %+v", v)
	}
}

// A finished match on another engine records it, is served from disk through
// that engine after a restart, and refuses to start a server that no longer
// links it.
func TestEngineRecordedAcrossRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := host.New(scriptOptions(dir, scriptEngine{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(scriptTable()); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("s"); err != nil {
		t.Fatal(err)
	}
	playScript(t, r)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if sc := readJSONFields(t, filepath.Join(dir, "s", "1.json")); string(sc["engine"]) != `"script"` {
		t.Fatalf("sidecar engine %s", sc["engine"])
	}

	again, err := host.New(scriptOptions(dir, scriptEngine{}))
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if _, err := again.ViewAt("s", 1, 2); err != nil {
		t.Fatalf("archived view: %v", err)
	}

	_, err = host.New(scriptOptions(dir, nil))
	if err == nil || !strings.Contains(err.Error(), `engine "script"`) || !strings.Contains(err.Error(), "available: gorge") {
		t.Fatalf("restart without the engine: %v", err)
	}
}

func TestEmptyEngineIsGorge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := host.New(scriptOptions(dir, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for id, name := range map[host.TableID]string{"empty": "", "named": gorgegame.Name} {
		cfg := host.TableConfig{ID: id, Seats: 2, Decks: []string{"a", "b"}, Spectator: view.Public, Engine: name}
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
	}
	for _, info := range r.Tables() {
		if info.Engine != "" {
			t.Fatalf("table %s reports engine %q", info.ID, info.Engine)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "tables.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"engine"`) {
		t.Fatalf("tables.json names an engine: %s", raw)
	}
}

func TestUnknownEngineRefused(t *testing.T) {
	t.Parallel()
	r, err := host.New(scriptOptions("", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := scriptTable()
	cfg.Engine = "missing"
	err = r.AddTable(cfg)
	if err == nil || !strings.Contains(err.Error(), `engine "missing"`) || !strings.Contains(err.Error(), "available: gorge") {
		t.Fatalf("AddTable: %v", err)
	}
}

func TestEngineAcceptsRunsAtAddTable(t *testing.T) {
	t.Parallel()
	r, err := host.New(scriptOptions("", scriptEngine{refuse: errors.New(`deck "a": card "X" is not supported`)}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	err = r.AddTable(scriptTable())
	if err == nil || !strings.Contains(err.Error(), `card "X" is not supported`) {
		t.Fatalf("AddTable: %v", err)
	}
	if len(r.Tables()) != 0 {
		t.Fatalf("a refused table registered: %+v", r.Tables())
	}
}

// A gorge match's sidecar names no engine, and loads after a restart.
func TestSidecarOmitsEngineForGorge(t *testing.T) {
	t.Parallel()
	names, decks := testutil.SampleDecks(t, 2)
	byName := map[string][]*cards.Card{}
	for i, n := range names {
		byName[n] = decks[i]
	}
	dir := t.TempDir()
	o := host.Options{
		Dir:      dir,
		LoadDeck: func(name string) (host.Deck, error) { return host.Deck{Name: name, Cards: byName[name]}, nil },
		Sleep:    func(time.Duration, <-chan struct{}) {},
	}
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	cfg := host.TableConfig{ID: "g", Seats: 2, Decks: names, Seed: 11, Spectator: view.Public}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("g"); err != nil {
		t.Fatal(err)
	}
	r.Wait("g")
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	sc := readJSONFields(t, filepath.Join(dir, "g", "1.json"))
	for _, key := range []string{"engine", "engine_key"} {
		if v, ok := sc[key]; ok {
			t.Fatalf("sidecar carries %s: %s", key, v)
		}
	}

	again, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if _, err := again.ViewAt("g", 1, 0); err != nil {
		t.Fatalf("archived view: %v", err)
	}
}

func readJSONFields(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}
