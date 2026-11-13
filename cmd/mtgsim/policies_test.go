package main

import (
	"bytes"
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
