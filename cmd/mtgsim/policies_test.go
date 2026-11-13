package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRunPlaysEveryKnownPolicy is the regression test for mtgsim building
// named-policy seats without the card registry: every policy bots.Names()
// lists must actually play a game when selected with -a. sb-tactical reads
// printed card facts from Deps.Cards and refuses a nil registry, so a seat
// built without it fails before the first decision.
func TestRunPlaysEveryKnownPolicy(t *testing.T) {
	names := bots.Names()
	if len(names) < 6 {
		t.Fatalf("bots.Names() has %d policies (%v), want at least 6 linked by bots/all", len(names), names)
	}
	found := false
	for _, name := range names {
		if name == "sb-tactical" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sb-tactical is not among bots.Names() (%v); bots/all no longer links it", names)
	}

	dir := testutil.CorpusDirOrSkip(t)
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := runAgents("", 2, 0, 1, true, false, dir, name, "bot", nil, &buf); err != nil {
				t.Fatalf("runAgents with -a %s: %v\n%s", name, err, buf.String())
			}
			out := buf.String()
			if !strings.Contains(out, "replay OK") {
				t.Fatalf("policy %s did not replay:\n%s", name, out)
			}
			if !strings.Contains(out, "winner=") && !strings.Contains(out, "draw") {
				t.Fatalf("policy %s produced no result:\n%s", name, out)
			}
			if strings.Contains(out, "FAILED") {
				t.Fatalf("policy %s failed:\n%s", name, out)
			}
		})
	}
}

// TestPoliciesFlagParse pins -policies: one entry per seat from seat 0,
// "-" or empty for a default seat, a shorter list padded with defaults, and
// a list longer than -seats or an unknown name refused as a usage error.
// Without -policies, -a and -b name seats 0 and 1.
func TestPoliciesFlagParse(t *testing.T) {
	for _, tc := range []struct {
		name  string
		list  []string
		a, b  string
		seats int
		want  []string
		usage bool
	}{
		{name: "a and b", a: "lethal-pressure", b: "bot", seats: 4, want: []string{"lethal-pressure", "bot", "", ""}},
		{name: "a only, one seat", a: "", seats: 1, want: []string{""}},
		{name: "every seat", list: []string{"az-redeal", "lethal-pressure", "-", "forge-heuristic"}, seats: 4,
			want: []string{"az-redeal", "lethal-pressure", "", "forge-heuristic"}},
		{name: "beats a and b", list: []string{"-", " bot "}, a: "lethal-pressure", b: "forge-heuristic", seats: 3,
			want: []string{"", "bot", ""}},
		{name: "too long", list: []string{"bot", "bot", "bot"}, seats: 2, usage: true},
		{name: "unknown", list: []string{"bot", "no-such-policy"}, seats: 2, usage: true},
		{name: "unknown via -a", a: "no-such-policy", seats: 2, usage: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := seatPolicies(tc.list, tc.a, tc.b, tc.seats)
			if tc.usage {
				var ue usageError
				if !errors.As(err, &ue) {
					t.Fatalf("seatPolicies = %v, %v; want a usage error", got, err)
				}
				return
			}
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("seatPolicies = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// TestPoliciesFourSeat: a four-seat Commander pod seats every -policies
// entry, names them on the summary line, and replays; an over-long list is
// a usage error before any game.
func TestPoliciesFourSeat(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	o := simOptions{Seats: 4, Games: 1, Verify: true, Dir: dir, Format: "commander",
		Policies: []string{"lethal-pressure", "-", "forge-heuristic", "bot"}}
	var buf bytes.Buffer
	if err := runSim(o, &buf); err != nil {
		t.Fatalf("runSim: %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{"4 seats,", "policies=lethal-pressure,-,forge-heuristic,bot\n", "replay OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	o.Policies = []string{"bot", "bot", "bot", "bot", "bot"}
	buf.Reset()
	var ue usageError
	if err := runSim(o, &buf); !errors.As(err, &ue) {
		t.Fatalf("five policies for four seats: err %v, want a usage error", err)
	}
}

// TestEnvSeatSearchesAsTheBenchDoes: an az-redeal seat, driven through
// mtgsim's seat route, searches (it used to answer Decide, its wrapped bot,
// at every decision) and plays exactly the game testkit/bench.PlayGame
// plays with Hooks.EnvSeats, the driver that mirrors the host's Env
// dispatch. The search budget is cut so the game fits the test budget.
func TestEnvSeatSearchesAsTheBenchDoes(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := testutil.LegacyDeckNames()[:2]
	decks := make([][]*cards.Card, 2)
	for i := range decks {
		if decks[i], err = testutil.LoadRepoDeck(reg, names[i]); err != nil {
			t.Fatal(err)
		}
	}
	const seed = 3
	newSeats := func() []engineseat.Seat {
		cfg := azredeal.Hosted()
		cfg.Search.Sims = 8
		az, err := azredeal.New(bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: reg}}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		return []engineseat.Seat{az, heuristic.NewBot(seed)}
	}
	cfg := gameConfig(seed, names, decks, reg, 0, nil, 0)
	cfg.LoopGuard = rules.WithDefaultObjectCap(cfg.LoopGuard)

	searched := 0
	prev := azmcts.Watch
	azmcts.Watch = func(d azmcts.Diag) {
		if d.Searched {
			searched++
		}
	}
	defer func() { azmcts.Watch = prev }()

	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	seats := newSeats()
	if _, err := driveMatch(e, seats, newSeatRoute(cfg, seats), nil); err != nil {
		t.Fatalf("driveMatch: %v", err)
	}
	if !e.G.Over {
		t.Fatal("the mtgsim game did not finish")
	}
	if searched == 0 {
		t.Fatal("the az-redeal seat never searched through mtgsim's route")
	}
	got := searched

	searched = 0
	_, be, err := bench.PlayGame(cfg, newSeats(), 0, maxIntents, bench.Hooks{
		EnvSeats:  true,
		NewEngine: rules.NewStartingPlayerChoice,
		Setup:     func(e *rules.Engine) { e.AskStartingPlayer() },
	})
	if err != nil {
		t.Fatalf("bench.PlayGame: %v", err)
	}
	if h, bh := e.L.Head(), be.L.Head(); h != bh || searched != got {
		t.Fatalf("mtgsim chain %s (%d searched), bench chain %s (%d searched)", h, got, bh, searched)
	}
	t.Logf("%d searched decisions, chain %s", got, e.L.Head())
}
