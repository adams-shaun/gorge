package rules

// cr-g229: Forge's `DB$ Cleanup | ClearNamedCard$ True` must wipe the host's
// chosen card name when the ability resolves, so a repeatable "choose a card
// name" ability (Wood Sage) names afresh on its next activation instead of
// silently reusing the first name (CR 201.4).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cleanNamedEngine builds a two-seat game whose seat-0 deck leads with the
// corpus Wood Sage and whose Config carries the compiled corpus as the name
// universe (without it effNameCard takes the no-ask legacy path and this test
// could not observe a name event at all).
func cleanNamedEngine(t *testing.T, reg *cards.Registry) (*Engine, Config) {
	t.Helper()
	mountain := lookup(t, reg, "Mountain")
	sage := lookup(t, reg, "Wood Sage")
	fill := func(extras []*cards.Card) []*cards.Card {
		deck := append([]*cards.Card{}, extras...)
		for len(deck) < 40 {
			deck = append(deck, mountain)
		}
		return deck
	}
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks:        [][]*cards.Card{fill([]*cards.Card{sage}), fill(nil)},
		NameUniverse: reg.Universe()})
	e := New(cfg)
	if e.G.NameUniverse.Len() == 0 {
		t.Fatal("precondition: NameUniverse not wired from Config")
	}
	e.Advance()
	return e, cfg
}

// cleanNamedDrain answers "pass" while Wood Sage's ability resolves, choosing
// name for its name ask, and returns how many name asks it answered.
func cleanNamedDrain(t *testing.T, e *Engine, name string) int {
	t.Helper()
	names := 0
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending while resolving the Wood Sage ability (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "name" {
			idx := labelIndex(d, name)
			if idx < 0 {
				t.Fatalf("name ask did not offer %q", name)
			}
			submitChoices(t, e, idx)
			names++
			continue
		}
		passed := false
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				passed = true
				break
			}
		}
		if !passed {
			t.Fatalf("no pass option while resolving the Wood Sage ability: %+v", d)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("Wood Sage's ability did not leave the stack")
	}
	return names
}

// TestWoodSageNamesAnewEachActivation: Wood Sage's {T} NameCard chain ends in
// `DB$ Cleanup | ClearNamedCard$ True`, so the chosen name is gone after the
// first resolution (not persisted on the permanent) and the second activation
// poses and records its own name ask.
func TestWoodSageNamesAnewEachActivation(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	card := lookup(t, reg, "Wood Sage")
	cleanup, ok := card.Faces[0].SVars["DBCleanup"]
	if !ok {
		t.Fatal("corpus precondition: Wood Sage DBCleanup SVar missing")
	}
	if !strings.Contains(cleanup, "ClearNamedCard$ True") {
		t.Fatalf("corpus precondition: DBCleanup = %q, want ClearNamedCard$ True", cleanup)
	}
	if lookup(t, reg, "Grizzly Bears") == nil || lookup(t, reg, "Hill Giant") == nil {
		t.Fatal("corpus precondition: the two chosen creature names must exist")
	}

	e, cfg := cleanNamedEngine(t, reg)
	sage := moveCorpusCard(t, e, "Wood Sage", 0, state.ZBattlefield)
	if got := e.G.Obj(sage).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: Wood Sage zone = %s, want battlefield", got)
	}

	// Its first activation needs an untapped, non-summoning-sick permanent.
	driveToStep(t, e, 3, 0, state.StepMain1)
	submitChoices(t, e, abilityOption(t, e, sage, 0).Index)
	if n := cleanNamedDrain(t, e, "Grizzly Bears"); n != 1 {
		t.Fatalf("first activation answered %d name asks, want 1", n)
	}
	if got := e.G.Obj(sage).ChosenName; got != "" {
		t.Fatalf("ChosenName after the first resolution = %q, want empty", got)
	}
	if n := countChooseNames(e, sage); n != 1 {
		t.Fatalf("name events after the first activation = %d, want 1", n)
	}

	driveToStep(t, e, 5, 0, state.StepMain1)
	submitChoices(t, e, abilityOption(t, e, sage, 0).Index)
	if n := cleanNamedDrain(t, e, "Hill Giant"); n != 1 {
		t.Fatalf("second activation answered %d name asks, want 1", n)
	}
	if got := e.G.Obj(sage).ChosenName; got != "" {
		t.Fatalf("ChosenName after the second resolution = %q, want empty", got)
	}
	if n := countChooseNames(e, sage); n != 2 {
		t.Fatalf("name events after the second activation = %d, want 2", n)
	}
	last := ""
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Counter == "name" && ev.Obj == sage {
			last = ev.Text
		}
	}
	if last != "Hill Giant" {
		t.Fatalf("last name event = %q, want the second activation's Hill Giant", last)
	}
	replayCheck(t, e, cfg)
}

func countChooseNames(e *Engine, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Counter == "name" && ev.Obj == id {
			n++
		}
	}
	return n
}
