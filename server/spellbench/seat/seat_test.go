package seat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestRegisterMakesATwoSeatExperimentalPolicy(t *testing.T) {
	const name = "sb-seat-test-policy"
	Register(name, []string{"/bin/true"})
	entry, ok := bots.Lookup(name)
	if !ok {
		t.Fatalf("bots.Lookup(%q) missing after Register", name)
	}
	if entry.Tier != bots.Experimental {
		t.Errorf("tier = %q, want experimental", entry.Tier)
	}
	if entry.MaxSeats != 2 {
		t.Errorf("max seats = %d, want 2", entry.MaxSeats)
	}
	if !entry.NeedsAutoPay {
		t.Error("NeedsAutoPay is false; the v2 wire only offers automatic payment")
	}
	if len(entry.Formats) != 1 || entry.Formats[0] != "constructed" {
		t.Errorf("formats = %v, want [constructed]", entry.Formats)
	}
}

func TestParseSpec(t *testing.T) {
	name, argv, err := ParseSpec("mini=/usr/bin/python3 bot.py")
	if err != nil {
		t.Fatal(err)
	}
	if name != "mini" || len(argv) != 1 || argv[0] != "/usr/bin/python3 bot.py" {
		t.Fatalf("ParseSpec = %q %v", name, argv)
	}
	name, argv, err = ParseSpec("mini=/bin/true")
	if err != nil || name != "mini" || len(argv) != 1 || argv[0] != "/bin/true" {
		t.Fatalf("ParseSpec(/bin/true) = %q %v %v", name, argv, err)
	}
	if _, _, err := ParseSpec("mini"); err == nil {
		t.Fatal("ParseSpec accepted a spec without =")
	}
	if _, _, err := ParseSpec("=/bin/true"); err == nil {
		t.Fatal("ParseSpec accepted an empty name")
	}
}

func TestV2AgentSeatPlaysMinimalBot(t *testing.T) {
	argv, record := recordingMinimal(t)
	for _, seed := range []uint64{7, 8, 9} {
		e, cfg, _ := testEngineSeed(t, seed)
		agent, fallback := attachTestSeatArgv(t, e, 0, argv, 0)
		seats := []engineseat.Seat{agent, fallback}
		n, err := drive(t, e, seats, 20000)
		if err != nil {
			t.Fatalf("seed %d: drive after %d intents: %v", seed, n, err)
		}
		if !e.G.Over || n < 10 || agent.forfeit != "" {
			t.Fatalf("seed %d: over=%v intents=%d forfeit=%q", seed, e.G.Over, n, agent.forfeit)
		}
		if _, err := replay.Replay(e.L, cfg); err != nil {
			t.Fatalf("seed %d: replay: %v", seed, err)
		}
		endTestMatch(seats)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"request_type":"choose"`) {
		t.Fatal("minimal bot recorded no choose requests")
	}
	if strings.Contains(string(raw), `"kind":"activate_mana_ability"`) {
		t.Fatal("minimal bot was sent a mana-tap candidate; the seat must use automatic payment")
	}
}

func TestV2AgentSeatPlaysGorgeAgent(t *testing.T) {
	goCmd := goRun(t)
	record := filepath.Join(t.TempDir(), "choose.jsonl")
	t.Setenv("SPELLBENCH_RECORD", record)
	for _, seed := range []uint64{7, 8, 9} {
		e, cfg, _ := testEngineSeed(t, seed)
		agent, fallback := attachTestSeatArgv(t, e, 0, append(append([]string(nil), goCmd...), "./testdata/gorgeagent"), 0)
		seats := []engineseat.Seat{agent, fallback}
		n, err := drive(t, e, seats, 20000)
		if err != nil {
			t.Fatalf("seed %d: drive after %d intents: %v", seed, n, err)
		}
		if !e.G.Over || n < 10 || agent.forfeit != "" {
			t.Fatalf("seed %d: over=%v intents=%d forfeit=%q", seed, e.G.Over, n, agent.forfeit)
		}
		if _, err := replay.Replay(e.L, cfg); err != nil {
			t.Fatalf("seed %d: replay: %v", seed, err)
		}
		endTestMatch(seats)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"request_type":"choose"`) {
		t.Fatal("helper agent recorded no choose requests")
	}
	if strings.Contains(string(raw), `"kind":"activate_mana_ability"`) {
		t.Fatal("agent was sent a mana-tap candidate; the seat must use automatic payment")
	}
}

func TestV2AgentSeatForfeitsOnTimeout(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("SPELLBENCH_MARKER", marker)
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "silent_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, v := nextAgentAsk(t, e, agent, fallback)
	ctx := newFiredDeadline()
	var (
		in  decision.Intent
		err error
	)
	done := make(chan struct{})
	go func() {
		in, err = agent.Decide(ctx, v, d)
		close(done)
	}()
	awaitMarker(t, marker)
	ctx.fire()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Decide did not return after the deadline fired")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("timeout produced %+v, want a concede", in)
	}
	if agent.forfeit != forfeitTimeout {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitTimeout)
	}
}

func TestV2AgentSeatForfeitsOnCrash(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "crash_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, in, err := nextAgentDecision(t, e, agent, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("crash produced %+v, want a concede", in)
	}
	if agent.forfeit != forfeitTransport {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitTransport)
	}
}

func TestV2AgentSeatForfeitsOnBadCandidate(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "bad_candidate_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, in, err := nextAgentDecision(t, e, agent, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("bad candidate produced %+v, want a concede", in)
	}
	if agent.forfeit != forfeitInvalid {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitInvalid)
	}
}

func TestV2AgentSeatForfeitsOnAgentError(t *testing.T) {
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "error_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent, fallback})

	d, in, err := nextAgentDecision(t, e, agent, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if !isConcede(d, in) {
		t.Fatalf("agent error produced %+v, want a concede", in)
	}
	if agent.forfeit != forfeitAgent {
		t.Fatalf("forfeit = %q, want %q", agent.forfeit, forfeitAgent)
	}
}

func testEngine(t *testing.T) (*rules.Engine, rules.Config, []string) {
	t.Helper()
	return testEngineSeed(t, 7)
}

func testEngineSeed(t *testing.T, seed uint64) (*rules.Engine, rules.Config, []string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck, ok := catalog.ByID("Burn")
	if !ok {
		t.Fatal("catalog deck Burn is missing")
	}
	burn, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Burn", "Burn"}
	decks := [][]*cards.Card{burn, burn}
	cfg := rules.Config{
		Seed:         seed,
		Names:        names,
		Decks:        decks,
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
	}
	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	e.Advance()
	return e, cfg, names
}

func attachTestSeatScript(t *testing.T, e *rules.Engine, idx int, script string, deadlineMS int) (*Agent, engineseat.Seat) {
	t.Helper()
	python(t)
	return attachTestSeatArgv(t, e, idx, []string{script}, deadlineMS)
}

func attachTestSeatArgv(t *testing.T, e *rules.Engine, idx int, argv []string, deadlineMS int) (*Agent, engineseat.Seat) {
	t.Helper()
	fallback := heuristic.NewBot(uint64(idx + 1))
	raw, err := New("test-agent", argv, bots.Options{Seed: uint64(idx + 1)})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	mc := &bots.MatchContext{
		Context:      context.Background(),
		Game:         gorgegame.Game{E: e},
		GameID:       "g-0000000000000001",
		Seat:         idx,
		Seats:        2,
		Format:       "constructed",
		Seed:         []byte("test"),
		OwnDeck:      e.OwnDeck(state.PlayerID(idx)),
		Mulligan:     "none",
		StartingSeat: int(e.G.Active),
		DeadlineMS:   deadlineMS,
		Logf:         t.Logf,
	}
	if err := agent.AttachMatch(mc); err != nil {
		t.Fatalf("AttachMatch: %v", err)
	}
	return agent, fallback
}

func nextAgentDecision(t *testing.T, e *rules.Engine, agent *Agent, fallback engineseat.Seat) (decision.Decision, decision.Intent, error) {
	t.Helper()
	d, v := nextAgentAsk(t, e, agent, fallback)
	in, err := agent.Decide(context.Background(), v, d)
	return d, in, err
}

func nextAgentAsk(t *testing.T, e *rules.Engine, agent *Agent, fallback engineseat.Seat) (decision.Decision, view.View) {
	t.Helper()
	for i := 0; i < 10000; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("engine has no pending decision")
		}
		v := view.Project(e.G, e, d.Player, d)
		v.Round = view.RoundOf(e.G, e.L.Events)
		if d.Player == 0 && d.Kind == decision.KPriority {
			return *d, v
		}
		in, err := fallback.Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("fallback at intent %d: %v", i, err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("fallback intent %d rejected: %v", i, err)
		}
	}
	t.Fatal("no seat-0 priority decision within 10000 intents")
	return decision.Decision{}, view.View{}
}

func drive(t *testing.T, e *rules.Engine, seats []engineseat.Seat, max int) (int, error) {
	t.Helper()
	n := 0
	for !e.G.Over && e.Pending() != nil {
		if n >= max {
			return n, nil
		}
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		v.Round = view.RoundOf(e.G, e.L.Events)
		in, err := seats[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			return n, err
		}
		if err := e.Submit(in); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func endTestMatch(seats []engineseat.Seat) {
	for _, s := range seats {
		bots.EndMatch(s, bots.MatchResult{Classification: "halted", Reason: "test"})
	}
}

func isConcede(d decision.Decision, in decision.Intent) bool {
	if len(in.Choices) != 1 || in.Choices[0] < 0 || in.Choices[0] >= len(d.Options) {
		return false
	}
	return d.Options[in.Choices[0]].Kind == "concede"
}

// recordingMinimal runs Jack's minimal_bot.py behind a tiny proxy that records
// the exact request lines the bot receives. The proxy keeps the example itself
// unmodified while letting the test assert the seat never offers mana taps.
func recordingMinimal(t *testing.T) (argv []string, record string) {
	t.Helper()
	py := python(t)
	record = filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv("SPELLBENCH_RECORD", record)
	t.Setenv("SPELLBENCH_MINIMAL", script(t, "minimal_bot.py"))
	wrapper := filepath.Join(t.TempDir(), "record_minimal.py")
	if err := os.WriteFile(wrapper, []byte(recordMinimalSrc), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{py, wrapper}, record
}

const recordMinimalSrc = `import os, subprocess, sys
bot = subprocess.Popen([sys.executable, os.environ["SPELLBENCH_MINIMAL"]],
                       stdin=subprocess.PIPE, stdout=subprocess.PIPE)
with open(os.environ["SPELLBENCH_RECORD"], "ab") as record:
    try:
        for line in sys.stdin.buffer:
            record.write(line)
            record.flush()
            bot.stdin.write(line)
            bot.stdin.flush()
            reply = bot.stdout.readline()
            if not reply:
                break
            sys.stdout.buffer.write(reply)
            sys.stdout.buffer.flush()
    finally:
        try:
            bot.stdin.close()
        except OSError:
            pass
        bot.wait()
`

func script(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func python(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	return p
}

func goRun(t *testing.T) []string {
	t.Helper()
	p, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not installed")
	}
	return []string{p, "run"}
}

// After a host undo the seat reads the replayed game: its environment holds the
// new board, what it had open is gone, and it answers the rewound decision.
func TestAgentFollowsARewind(t *testing.T) {
	e, cfg, _ := testEngine(t)
	agent, fallback := attachTestSeatScript(t, e, 0, script(t, "minimal_bot.py"), 0)
	defer endTestMatch([]engineseat.Seat{agent})
	var at int
	for range 3 {
		_, in, err := nextAgentDecision(t, e, agent, fallback)
		if err != nil {
			t.Fatal(err)
		}
		at = len(e.L.Intents)
		if err := e.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	rewound, err := replay.ReplayTo(e.L.Clone(), cfg, at)
	if err != nil {
		t.Fatal(err)
	}
	agent.tx = stuckTransaction{}
	agent.MatchRewound(gorgegame.Game{E: rewound})
	if agent.tx != nil {
		t.Fatal("a transaction survived the rewind")
	}
	if agent.env.G.R.Board() != rewound.G {
		t.Fatal("the seat still reads the game it attached to")
	}

	d := rewound.Pending()
	v := view.Project(rewound.G, rewound, d.Player, d)
	in, err := agent.Decide(context.Background(), v, *d)
	if err != nil {
		t.Fatal(err)
	}
	if agent.Forfeit() != "" {
		t.Fatalf("forfeit %s after the rewind", agent.Forfeit())
	}
	if err := rewound.Submit(in); err != nil {
		t.Fatalf("the rewound game refused the seat's answer: %v", err)
	}
}

// stuckTransaction stands for a transaction left open across a rewind.
type stuckTransaction struct{ mapping.Transaction }
