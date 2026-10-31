package seat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestDeckWireUsesCanonicalDeckID(t *testing.T) {
	m := &deck.Manifest{Name: "burn", Main: []deck.ManifestRow{
		{Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 18},
	}}
	got := deckWire(m)
	want := wire.DeckID([]wire.DeckRow{
		{Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 18},
	})
	if got.DeckID != want {
		t.Fatalf("deck_id = %s, want canonical %s", got.DeckID, want)
	}
}

func TestGameStartProfileCoversPosedDecisionKinds(t *testing.T) {
	argv, record := recordingMinimal(t)
	e, _ := testEngineStartingChoice(t, 7)
	d := e.Pending()
	idx := int(d.Player)
	agent, _ := attachTestSeatArgv(t, e, idx, argv, 0)
	seats := []engineseat.Seat{heuristic.NewBot(1), heuristic.NewBot(2)}
	seats[idx] = agent
	n, err := drive(t, e, seats, 20000)
	if err != nil {
		t.Fatalf("drive after %d intents: %v", n, err)
	}
	if !e.G.Over || agent.forfeit != "" {
		t.Fatalf("over=%v forfeit=%q", e.G.Over, agent.forfeit)
	}
	endTestMatch(seats)

	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	profile := map[string]bool{}
	sawStarting := false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var req struct {
			RequestType   string `json:"request_type"`
			EngineProfile struct {
				DecisionKinds []string `json:"decision_kinds"`
			} `json:"engine_profile"`
			Decision struct {
				Candidates []struct {
					Semantic struct {
						Kind string `json:"kind"`
					} `json:"semantic"`
				} `json:"candidates"`
			} `json:"decision"`
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatalf("request line is not JSON: %v", err)
		}
		switch req.RequestType {
		case "game_start":
			for _, k := range req.EngineProfile.DecisionKinds {
				profile[k] = true
			}
		case "choose":
			for _, c := range req.Decision.Candidates {
				if !profile[c.Semantic.Kind] {
					t.Errorf("choose poses %q but game_start does not declare it; profile=%v", c.Semantic.Kind, profile)
				}
				if c.Semantic.Kind == "choose_starting_player" {
					sawStarting = true
				}
			}
		}
	}
	if len(profile) == 0 {
		t.Fatal("no game_start profile was recorded")
	}
	if !profile["choose_starting_player"] {
		t.Fatalf("game_start profile omits choose_starting_player: %v", profile)
	}
	if !sawStarting {
		t.Fatal("the seat never posed the starting-player choice it advertises")
	}
}

func TestStartupContextArmsStartupDeadline(t *testing.T) {
	raw, err := New("test-agent", []string{"/bin/true"}, bots.Options{})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	agent.ctx = context.Background()
	ctx, cancel := agent.startupContext()
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("startup handshake context has no deadline")
	}
	if remaining := time.Until(deadline); remaining < 299*time.Second || remaining > 301*time.Second {
		t.Fatalf("startup deadline is %v away, want ~%d ms", remaining, startupMS)
	}
}

func TestStartupHandshakeTimeoutForfeits(t *testing.T) {
	e, _, _ := testEngine(t)
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("SPELLBENCH_MARKER", marker)
	ctx := newFiredDeadline()
	go func() {
		waitMarker(marker, 10*time.Second)
		ctx.fire()
	}()
	agent, fallback := attachTestSeatContext(t, e, 0, []string{script(t, "silent_hello_bot.py")}, ctx)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("helper agent never read hello: %v", err)
	}
	if agent.forfeit != forfeitTimeout {
		t.Fatalf("startup forfeit = %q, want %q", agent.forfeit, forfeitTimeout)
	}
	d, v := nextAgentAsk(t, e, agent, fallback)
	in, err := agent.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("startup timeout produced %+v, want a concede", in)
	}
}

func TestV2AgentSeatForfeitsOnMalformedLine(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "malformed_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, in, err := nextAgentDecision(t, e, agent, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("malformed line produced %+v, want a concede", in)
	}
	if agent.forfeit != forfeitAgent {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitAgent)
	}
}

func TestV2AgentSeatForfeitsOnMissingSelection(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "no_selection_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, in, err := nextAgentDecision(t, e, agent, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("missing selection produced %+v, want a concede", in)
	}
	if agent.forfeit != forfeitAgent {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitAgent)
	}
}

func TestV2AgentSeatDefaultsNonPriorityAfterForfeit(t *testing.T) {
	e, _ := testEngineStartingChoice(t, 7)
	d := e.Pending()
	idx := int(d.Player)
	agent, _ := attachTestSeatScript(t, e, idx, script(t, "error_bot.py"), 0)
	v := view.Project(e.G, e, d.Player, d)
	v.Round = view.RoundOf(e.G, e.L.Events)
	in, err := agent.Decide(context.Background(), v, *d)
	if err != nil {
		t.Fatal(err)
	}
	if isConcede(*d, in) {
		t.Fatalf("non-priority forfeit answered %s with a concede", d.Kind)
	}
	if agent.forfeit != forfeitAgent {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitAgent)
	}
	if !agent.fallbackLogged {
		t.Fatal("non-priority fallback was silent")
	}
	if err := e.Submit(in); err != nil {
		t.Fatalf("fallback intent rejected: %v", err)
	}
	seats := []engineseat.Seat{heuristic.NewBot(1), heuristic.NewBot(2)}
	seats[idx] = agent
	n, err := drive(t, e, seats, 20000)
	if err != nil {
		t.Fatalf("drive after %d intents: %v", n, err)
	}
	if !e.G.Over {
		t.Fatalf("game did not end after forfeit (intents=%d)", n)
	}
	endTestMatch(seats)
}

func TestUnattachedAgentDecideFailsLoudly(t *testing.T) {
	raw, err := New("test-agent", []string{"/bin/true"}, bots.Options{})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	if _, err := agent.Decide(context.Background(), view.View{}, decision.Decision{}); err == nil {
		t.Fatal("unattached agent silently used the heuristic fallback")
	}
}

func TestAttachMatchRequiresEngineAndOwnDeck(t *testing.T) {
	raw, err := New("test-agent", []string{"/bin/true"}, bots.Options{})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	if err := agent.AttachMatch(&bots.MatchContext{Seats: 2, OwnDeck: &deck.Manifest{}}); err == nil {
		t.Fatal("AttachMatch accepted a nil game")
	}
	if err := agent.AttachMatch(&bots.MatchContext{Seats: 2, Game: gorgegame.Game{E: &rules.Engine{}}}); err == nil {
		t.Fatal("AttachMatch accepted a nil own deck")
	}
}

func testEngineStartingChoice(t *testing.T, seed uint64) (*rules.Engine, rules.Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	d, ok := catalog.ByID("Burn")
	if !ok {
		t.Fatal("catalog deck Burn is missing")
	}
	burn, err := catalog.Resolve(reg, d)
	if err != nil {
		t.Fatal(err)
	}
	cfg := rules.Config{
		Seed:         seed,
		Names:        []string{"Burn", "Burn"},
		Decks:        [][]*cards.Card{burn, burn},
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
	}
	e := rules.NewStartingPlayerChoice(cfg)
	if e.AskStartingPlayer() == nil {
		t.Fatal("no starting-player choice was posed")
	}
	return e, cfg
}

func attachTestSeatContext(t *testing.T, e *rules.Engine, idx int, argv []string, ctx context.Context) (*Agent, engineseat.Seat) {
	t.Helper()
	python(t)
	fallback := heuristic.NewBot(uint64(idx + 1))
	raw, err := New("test-agent", argv, bots.Options{Seed: uint64(idx + 1)})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	mc := &bots.MatchContext{
		Context:      ctx,
		Game:         gorgegame.Game{E: e},
		GameID:       "g-0000000000000001",
		Seat:         idx,
		Seats:        2,
		Format:       "constructed",
		Seed:         []byte("test"),
		OwnDeck:      e.OwnDeck(state.PlayerID(idx)),
		Mulligan:     "none",
		StartingSeat: int(e.G.Active),
		Logf:         t.Logf,
	}
	if err := agent.AttachMatch(mc); err != nil {
		t.Fatalf("AttachMatch: %v", err)
	}
	return agent, fallback
}
