package bench_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots/spellbench"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"

	gbench "github.com/adams-shaun/gorge/testkit/bench"
)

// The shared drive loop arms the default object cap (rules.DefaultMaxObjs)
// on every config that sets no explicit budget, and a game that trips a
// volume reason ends as a recorded bigboard stall, never an engine-bug
// livelock abort.

// tokenDecks is a deck pair whose games mint tokens mid-resolution: 30
// Dragon Fodder (two 1/1 red Goblins in ONE resolution) and 30 Mountains per
// side, the same shape cmd/cardfuzz's boardcap fixture stages.
func tokenDecks(t *testing.T, reg *cards.Registry) [][]*cards.Card {
	t.Helper()
	fodder, ok := reg.Lookup("Dragon Fodder")
	if !ok {
		t.Fatal(`corpus card "Dragon Fodder" not found`)
	}
	mountain, ok := reg.Lookup("Mountain")
	if !ok {
		t.Fatal(`corpus card "Mountain" not found`)
	}
	d := make([]*cards.Card, 0, 60)
	for i := 0; i < 30; i++ {
		d = append(d, fodder, mountain)
	}
	return [][]*cards.Card{d, append([]*cards.Card(nil), d...)}
}

func botSeats(seed uint64, n int) []seat.Seat {
	out := make([]seat.Seat, n)
	for i := range out {
		out[i] = heuristic.NewBot(seed ^ uint64(i+1))
	}
	return out
}

// TestPlayGameArmsDefaultObjectCap pins the arming: a config with no
// LoopGuard plays under rules.DefaultMaxObjs, observed through the
// constructor the drive loop actually built the engine with, and an ordinary
// game under it finishes normally (the cap is a volume bound far above the
// measured ordinary high-water: 554).
func TestPlayGameArmsDefaultObjectCap(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var armed *rules.LoopGuard
	o, _, err := gbench.PlayGame(
		rules.Config{Seed: 7717, Names: []string{"p0", "p1"},
			Decks:  fdnPairDecks(t, reg),
			Tokens: reg.Tokens, NameUniverse: reg.Universe()},
		botSeats(7717, 2), 150, 40000,
		gbench.Hooks{NewEngine: func(c rules.Config) *rules.Engine {
			armed = c.LoopGuard
			return rules.New(c)
		}})
	if err != nil {
		t.Fatalf("ordinary game errored: %v", err)
	}
	if o.IsStalled() {
		t.Fatalf("ordinary game under the default cap stalled: %+v (%s)", o, o.Livelock)
	}
	if armed == nil || armed.MaxObjs != rules.DefaultMaxObjs {
		t.Fatalf("the drive loop armed %+v, want MaxObjs %d", armed, rules.DefaultMaxObjs)
	}
}

func fdnPairDecks(t *testing.T, reg *cards.Registry) [][]*cards.Card {
	t.Helper()
	cat, err := spellbench.CatalogByID("fdn")
	if err != nil {
		t.Fatal(err)
	}
	a, err := spellbench.Deck(reg, cat.Dir, cat.Pool[0])
	if err != nil {
		t.Fatal(err)
	}
	b, err := spellbench.Deck(reg, cat.Dir, cat.Pool[1])
	if err != nil {
		t.Fatal(err)
	}
	return [][]*cards.Card{a, b}
}

// TestPlayGameObjectCapStallIsBigboard pins the classification: a game that
// crosses an armed cap mid-resolution is recorded as a bigboard STALL (the
// vocabulary for degenerate volume), with the watcher's object-cap
// diagnostic and a bounded arena -- never as a "livelock" engine-bug abort.
func TestPlayGameObjectCapStallIsBigboard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const capObjs = 121
	o, e, err := gbench.PlayGame(
		rules.Config{Seed: 17, Names: []string{"p0", "p1"}, Decks: tokenDecks(t, reg),
			Tokens: reg.Tokens, NameUniverse: reg.Universe(),
			LoopGuard: &rules.LoopGuard{MaxObjs: capObjs}},
		botSeats(17, 2), 17, 20000, gbench.Hooks{})
	if err != nil {
		t.Fatalf("game errored: %v", err)
	}
	if o.StallOn != "bigboard" {
		t.Fatalf("StallOn %q, want bigboard (diag %s)", o.StallOn, o.Livelock)
	}
	if !strings.Contains(o.Livelock, "livelock detected (object cap") {
		t.Fatalf("diagnostic %q lacks the object-cap reason", o.Livelock)
	}
	// The arena must be bounded AT the cap: the abort fires one event after
	// the crossing, so the surviving arena is at most cap+1.
	if e == nil || len(e.G.Objs) > capObjs+1 {
		arena := -1
		if e != nil {
			arena = len(e.G.Objs)
		}
		t.Fatalf("arena %d not bounded by cap %d+1", arena, capObjs)
	}
	if o.Intents <= 0 {
		t.Fatalf("the game answered no decisions: %+v", o)
	}
}

// TestPlayGameObjectCapExplicitBudgetWins pins that a caller's own budget
// survives the default arming: a 121-object cap on the same token decks
// trips exactly like the default would, and the constructor saw 121 (not
// DefaultMaxObjs).
func TestPlayGameObjectCapExplicitBudgetWins(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var armed *rules.LoopGuard
	o, _, err := gbench.PlayGame(
		rules.Config{Seed: 17, Names: []string{"p0", "p1"}, Decks: tokenDecks(t, reg),
			Tokens: reg.Tokens, NameUniverse: reg.Universe(),
			LoopGuard: &rules.LoopGuard{MaxObjs: 121}},
		botSeats(17, 2), 17, 20000,
		gbench.Hooks{NewEngine: func(c rules.Config) *rules.Engine {
			armed = c.LoopGuard
			return rules.New(c)
		}})
	if err != nil {
		t.Fatalf("game errored: %v", err)
	}
	if armed == nil || armed.MaxObjs != 121 {
		t.Fatalf("explicit budget lost: %+v", armed)
	}
	if o.StallOn != "bigboard" || !strings.Contains(o.Livelock, "cap 121") {
		t.Fatalf("explicit cap did not trip as a bigboard stall: %+v %s", o, o.Livelock)
	}
}
