package host_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/forgeheuristic"
	"github.com/adams-shaun/gorge/bots/sbtactical"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const scriptName = "script"

// scriptEngine plays one game: each seat is asked one question, then seat 1
// wins.
type scriptEngine struct{ refuse error }

func (scriptEngine) Name() string { return scriptName }

func (e scriptEngine) Accepts(game.Config) error { return e.refuse }

func (scriptEngine) Caps() game.Caps { return game.Caps{Engine: scriptName} }

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

func (g *scriptGame) Caps() game.Caps { return scriptEngine{}.Caps() }

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

// scriptBotOptions serve bot seats that read an empty card registry: the script
// game holds no cards for them to look up.
func scriptBotOptions() host.Options {
	o := scriptOptions("", scriptEngine{})
	o.BotDeps = bots.Deps{Cards: &cards.Registry{}}
	return o
}

// scriptBotTable is scriptTable with policy in both seats.
func scriptBotTable(policy string) host.TableConfig {
	c := scriptTable()
	c.Humans = nil
	c.BotPolicy = policy
	c.AllowDecklistBots = true
	return c
}

// playToWinner runs table s to its end and fails unless seat 1 won its match.
func playToWinner(t *testing.T, r *host.Registry) {
	t.Helper()
	if err := r.Start("s"); err != nil {
		t.Fatal(err)
	}
	r.Wait("s")
	ms, err := r.Matches("s")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].State != protocol.MatchFinished || ms[0].Winner == nil || *ms[0].Winner != 1 {
		t.Fatalf("matches %+v", ms)
	}
}

func TestBotPoliciesPlayWithoutSearchRoots(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{host.BotPolicy, forgeheuristic.Policy, forgeheuristic.HoldPolicy, "lethal-pressure", sbtactical.Policy} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			r, err := host.New(scriptBotOptions())
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err := r.AddTable(scriptBotTable(policy)); err != nil {
				t.Fatal(err)
			}
			playToWinner(t, r)
		})
	}
}

func TestSearchPolicyRefusedWithoutSearchRoots(t *testing.T) {
	t.Parallel()
	r, err := host.New(scriptBotOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	bySeat := scriptBotTable("")
	bySeat.SeatBotPolicies = []string{"", azredeal.Policy}
	for _, cfg := range []host.TableConfig{scriptBotTable(azredeal.Policy), bySeat} {
		err := r.AddTable(cfg)
		if err == nil || !strings.Contains(err.Error(), strconv.Quote(azredeal.Policy)) || !strings.Contains(err.Error(), strconv.Quote(scriptName)) {
			t.Fatalf("AddTable: %v", err)
		}
	}
	if len(r.Tables()) != 0 {
		t.Fatalf("a refused table registered: %+v", r.Tables())
	}

	gorge, err := host.New(scriptOptions("", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer gorge.Close()
	cfg := scriptBotTable(azredeal.Policy)
	cfg.Engine = ""
	if err := gorge.AddTable(cfg); err != nil {
		t.Fatalf("gorge refused the policy: %v", err)
	}
}

// envRecorder keeps what the host hands the Env seat it wraps.
type envRecorder struct {
	bots.EnvSeat
	mc      *bots.MatchContext
	refused []string
}

func (s *envRecorder) AttachMatch(mc *bots.MatchContext) error {
	s.mc = mc
	return nil
}

func (s *envRecorder) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	if env.Search.Engine != nil {
		return decision.Intent{}, errors.New("a root from an engine that deals none")
	}
	s.refused = append(s.refused, env.RootRefused)
	return s.EnvSeat.DecideEnv(ctx, env, d)
}

func TestEnvSeatGetsRefusedRootWithoutSearchRoots(t *testing.T) {
	t.Parallel()
	o := scriptBotOptions()
	var recorders []*envRecorder
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		seats := make([]seat.Seat, len(names))
		for i := range names {
			s, err := bots.New(sbtactical.Policy, bots.Options{Seed: seed ^ uint64(i+1), Deps: o.BotDeps})
			if err != nil {
				t.Error(err)
				return seats
			}
			rec := &envRecorder{EnvSeat: s.(bots.EnvSeat)}
			recorders = append(recorders, rec)
			seats[i] = rec
		}
		return seats
	}
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(scriptBotTable(sbtactical.Policy)); err != nil {
		t.Fatal(err)
	}
	playToWinner(t, r)

	if len(recorders) != 2 {
		t.Fatalf("built %d seats", len(recorders))
	}
	for i, rec := range recorders {
		if rec.mc.Game == nil || rec.mc.Game.Caps().Engine != scriptName || rec.mc.Engine != nil {
			t.Fatalf("seat %d context: game %v, engine %v", i, rec.mc.Game, rec.mc.Engine)
		}
		if len(rec.refused) != 1 || !strings.Contains(rec.refused[0], "search roots") {
			t.Fatalf("seat %d refusals %q", i, rec.refused)
		}
	}
}

// closedListRecorder is an envRecorder that declares closed lists, the one kind
// of Env seat a table with a human may build.
type closedListRecorder struct{ envRecorder }

func (*closedListRecorder) SearchBelief() *searchprobe.Belief { return &searchprobe.Belief{} }

// At a table with a human the posed-resolution check reads gorge's engine, so
// an engine with no search roots must refuse the root before it.
func TestClosedListSeatGetsRefusedRootWithoutSearchRoots(t *testing.T) {
	t.Parallel()
	o := scriptBotOptions()
	inner, err := bots.New(sbtactical.Policy, bots.Options{Seed: 1, Deps: o.BotDeps})
	if err != nil {
		t.Fatal(err)
	}
	rec := &closedListRecorder{envRecorder{EnvSeat: inner.(bots.EnvSeat)}}
	o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{rec, rec} }
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := scriptTable()
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("s"); err != nil {
		t.Fatal(err)
	}
	d := awaitPending(t, r, 0)
	if err := r.SubmitIntent("s", 1, 0, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	r.Wait("s")
	if len(rec.refused) != 1 || !strings.Contains(rec.refused[0], "search roots") {
		t.Fatalf("refusals %q", rec.refused)
	}
}

// awaitPending polls until the human at seat p of table s has a decision.
func awaitPending(t *testing.T, r *host.Registry, p state.PlayerID) *decision.Decision {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		d, err := r.Pending("s", 1, p)
		if err == nil {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("no decision for seat %d: %v", p, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func wantEngineRefusal(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, host.ErrEngineUnsupported) || !strings.Contains(err.Error(), strconv.Quote(scriptName)) {
		t.Fatalf("got %v, want a refusal naming %q", err, scriptName)
	}
}

func TestFeedbackRefusesOtherEngines(t *testing.T) {
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
	refused := func(r *host.Registry) {
		t.Helper()
		reporter := state.PlayerID(0)
		_, err := r.SnapshotForFeedback("s", &reporter)
		wantEngineRefusal(t, err)
		_, err = r.SnapshotMatch("s", 1, nil)
		wantEngineRefusal(t, err)
	}
	awaitPending(t, r, 0)
	refused(r)
	playScript(t, r)
	refused(r)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := host.New(scriptOptions(dir, scriptEngine{}))
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	refused(again)
}

func TestShadowScanRefusesOtherEngines(t *testing.T) {
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
	playScript(t, r)
	_, err = r.ShadowScan("s", 1, 0, host.BotPolicy)
	wantEngineRefusal(t, err)

	// The ledger the scan reads is the host's own, so it stays readable.
	ledger, err := r.Provenance("s", 1)
	if err != nil || len(ledger) != 2 {
		t.Fatalf("provenance %v, %v; want one entry per intent", ledger, err)
	}
}

func TestReviewDecisionsRefuseOtherEngines(t *testing.T) {
	t.Parallel()
	r, err := host.New(scriptOptions("", scriptEngine{}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(scriptTable()); err != nil {
		t.Fatal(err)
	}
	_, err = r.ReviewDecisions("s", 1)
	wantEngineRefusal(t, err)
	if _, err := r.ReviewDecisions("missing", 1); !errors.Is(err, host.ErrNotFound) {
		t.Fatalf("missing table: %v", err)
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

// v2Answerer is a human at the far end of a v2 seat. The refusals below never
// reach it.
type v2Answerer struct{}

func (v2Answerer) Choose(context.Context, humanseat.Ask) (int, error) {
	return 0, errors.New("the seat was refused")
}

func (v2Answerer) NativeOnly(humanseat.NativeOnly) {}

// A v2 seat reads the game through a reader this engine does not offer: it
// refuses at attach, naming the engine, and the match ends with that reason
// before a decision is answered.
func TestV2SeatsRefuseAnEngineWithoutAReader(t *testing.T) {
	t.Parallel()
	const reason = `the "script" engine offers no v2 reader`
	crashed := func(t *testing.T, dir string, r *host.Registry) {
		t.Helper()
		if err := r.Start("s"); err != nil {
			t.Fatal(err)
		}
		r.Wait("s")
		ms, err := r.Matches("s")
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
			t.Fatalf("matches %+v", ms)
		}
		reports, err := filepath.Glob(filepath.Join(dir, "crash", "*"))
		if err != nil || len(reports) != 1 {
			t.Fatalf("crash reports %v: %v", reports, err)
		}
		report, err := os.ReadFile(reports[0])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(report), reason) {
			t.Fatalf("crash report:\n%s", report)
		}
	}
	t.Run("agent", func(t *testing.T) {
		t.Parallel()
		agent, err := sbseat.NewInProcess("v2", func(io.Reader, io.Writer) error {
			t.Error("the agent was started")
			return nil
		}, bots.Options{})
		if err != nil {
			t.Fatal(err)
		}
		o := scriptBotOptions()
		o.Dir = t.TempDir()
		o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{agent, agent} }
		r, err := host.New(o)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		if err := r.AddTable(scriptBotTable(host.BotPolicy)); err != nil {
			t.Fatal(err)
		}
		crashed(t, o.Dir, r)
	})
	t.Run("human", func(t *testing.T) {
		t.Parallel()
		o := scriptOptions(t.TempDir(), scriptEngine{})
		o.TapHuman = func(_ host.TableID, _ int, _ bool, mc *bots.MatchContext) (host.HumanTap, error) {
			return humanseat.New(humanseat.Config{Seats: mc.Seats, Format: mc.Format, Answerer: v2Answerer{}})
		}
		r, err := host.New(o)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		cfg := scriptTable()
		cfg.AutoMana, cfg.BotAutoPayMana = true, true
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
		crashed(t, o.Dir, r)
	})
}
