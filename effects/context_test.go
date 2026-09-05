package effects

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// fakeHost is the smallest thing satisfying Host: a real Game plus a captured
// event list, so effect tests assert on emitted events rather than internals.
// continuous captures every AddContinuous call the same way log captures
// every Emit call -- effects package tests have no rules.Engine (and no
// layer computation) to check the resulting Power/Toughness against, so they
// can only assert on what got registered, not on its downstream effect; the
// two engine-level end-to-end tests in rules/layers_pump_test.go are what
// check the actual computed result.
type fakeHost struct {
	g          *state.Game
	log        []events.Event
	continuous []state.ContinuousEffect
	n          int
}

func (h *fakeHost) Game() *state.Game { return h.g }
func (h *fakeHost) Emit(e events.Event) {
	h.log = append(h.log, e)
	events.Apply(h.g, e)
}
func (h *fakeHost) Rand(n int) int { h.n++; return 0 }
func (h *fakeHost) AddContinuous(ce state.ContinuousEffect) {
	h.continuous = append(h.continuous, ce)
}

// HasKeyword has no layer system to consult here (see the type doc comment),
// so it reads the printed face directly -- enough for the effects-package
// tests, which set up Indestructible by mutating Card.Faces[0].Keywords.
func (h *fakeHost) HasKeyword(id state.ObjID, kw string) bool {
	o := h.g.Obj(id)
	return o != nil && o.Face() != nil && o.Face().HasKeyword(kw)
}

func newHost(t *testing.T, seats int) *fakeHost {
	t.Helper()
	return &fakeHost{g: state.NewGame(names(seats))}
}

// fixtureHost builds a 2-seat game with two objects already on it -- object 1
// (ID from state.Game.AddObject's first call) controlled by seat 0, object 2
// controlled by seat 1 -- and a Ctx sourced at object 1. context_test.go's
// and count_test.go's own tests share this instead of each hand-rolling a
// board, the same way filter_test.go's board(t) is shared across that file.
func fixtureHost(t *testing.T) (*fakeHost, *Ctx) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	src := h.g.AddObject(card, 0)
	h.g.AddObject(card, 1)
	return h, &Ctx{Source: src.ID, Controller: 0}
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a' + i))
	}
	return out
}

func sa(t *testing.T, line string) *cards.SA {
	t.Helper()
	src := "Name:T\nTypes:Sorcery\nA:" + line + "\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	return c.Faces[0].Abilities[0]
}

func TestResolveWalksTheSubAbilityChain(t *testing.T) {
	var order []string
	Register("TestA", func(h Host, c *Ctx, s *cards.SA) { order = append(order, "A") })
	Register("TestB", func(h Host, c *Ctx, s *cards.SA) { order = append(order, "B") })
	Register("TestC", func(h Host, c *Ctx, s *cards.SA) { order = append(order, "C") })
	t.Cleanup(func() { unregister("TestA", "TestB", "TestC") })

	src := "Name:T\nTypes:Sorcery\nA:SP$ TestA | SubAbility$ X\nSVar:X:DB$ TestB | SubAbility$ Y\nSVar:Y:DB$ TestC\nOracle:x\n"
	c, _ := cards.ParseBytes("t.txt", []byte(src))
	c.Link()
	Resolve(newHost(t, 2), &Ctx{}, c.Faces[0].Abilities[0])

	if len(order) != 3 || order[0] != "A" || order[1] != "B" || order[2] != "C" {
		t.Fatalf("resolution order = %v", order)
	}
}

func TestResolveNotesUnimplementedAPIsInsteadOfPanicking(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ NoSuchApiExists | NumDmg$ 1"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("expected one Note event, got %+v", h.log)
	}
	if want := "unimplemented API NoSuchApiExists"; h.log[0].Text != want {
		t.Fatalf("Text = %q, want %q", h.log[0].Text, want)
	}
}

func TestDefinedResolvesEachForm(t *testing.T) {
	h := newHost(t, 4)
	ctx := &Ctx{Source: 7, Controller: 1,
		Targets:    []state.Target{{Player: 3, IsPlayer: true}},
		Remembered: []state.Target{{Obj: 9}}}

	check := func(line string, want []state.Target) {
		t.Helper()
		got := Defined(h, ctx, sa(t, line))
		if len(got) != len(want) {
			t.Fatalf("%s -> %v, want %v", line, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s -> %v, want %v", line, got, want)
			}
		}
	}
	check("SP$ X | Defined$ You", []state.Target{{Player: 1, IsPlayer: true}})
	check("SP$ X | Defined$ Self", []state.Target{{Obj: 7}})
	check("SP$ X | Defined$ Remembered", []state.Target{{Obj: 9}})
	check("SP$ X | Defined$ Targeted", []state.Target{{Player: 3, IsPlayer: true}})
	// No Defined$ and no ValidTgts$ falls back to the ability's own source
	// (object 7), not the chosen targets -- Forge's default (R-10).
	check("SP$ X | NumDmg$ 1", []state.Target{{Obj: 7}})
	// No Defined$ but ValidTgts$ present falls back to the chosen targets.
	check("SP$ X | ValidTgts$ Player | NumDmg$ 1", []state.Target{{Player: 3, IsPlayer: true}})
	// Opponent expands to every other living seat, in APNAP order from the
	// controller, so the resulting event order is deterministic.
	check("SP$ X | Defined$ Opponent", []state.Target{
		{Player: 2, IsPlayer: true}, {Player: 3, IsPlayer: true}, {Player: 0, IsPlayer: true}})
	// Player expands to every living seat including the controller, in APNAP
	// order from the controller.
	check("SP$ X | Defined$ Player", []state.Target{
		{Player: 1, IsPlayer: true}, {Player: 2, IsPlayer: true},
		{Player: 3, IsPlayer: true}, {Player: 0, IsPlayer: true}})
}

// TestDefinedReturnsCopiesNotAliases guards against Defined() handing back a
// slice that shares a backing array with Ctx.Targets or Ctx.Remembered.
// Ctx is threaded by pointer through Resolve, so if this aliased, the ordinary
// Go filter-in-place idiom applied to a Defined() result would corrupt state
// a later effect in the same Sub chain still relies on — silently, with no
// compiler warning. Covers every path in Defined that can return c.Targets or
// c.Remembered: the no-Defined$-but-ValidTgts$ case, Targeted, ParentTarget,
// Remembered, and the unknown-form fallback.
func TestDefinedReturnsCopiesNotAliases(t *testing.T) {
	h := newHost(t, 2)
	origTarget := state.Target{Player: 0, IsPlayer: true}
	origRemembered := state.Target{Obj: 5}

	lines := []string{
		"SP$ X | ValidTgts$ Creature | NumDmg$ 1", // no Defined$, has ValidTgts$: falls back to Targets
		"SP$ X | Defined$ Targeted",
		"SP$ X | Defined$ ParentTarget",
		"SP$ X | Defined$ SomeFormM1DoesNotModel", // unknown-form fallback
	}
	for _, line := range lines {
		ctx := &Ctx{Targets: []state.Target{origTarget}}
		got := Defined(h, ctx, sa(t, line))
		if len(got) != 1 {
			t.Fatalf("%s: got %v, want 1 target", line, got)
		}
		got[0] = state.Target{Player: 99, IsPlayer: true}
		if ctx.Targets[0] != origTarget {
			t.Fatalf("%s: mutating the Defined() result changed Ctx.Targets to %v", line, ctx.Targets)
		}
	}

	ctx := &Ctx{Remembered: []state.Target{origRemembered}}
	got := Defined(h, ctx, sa(t, "SP$ X | Defined$ Remembered"))
	if len(got) != 1 {
		t.Fatalf("Defined$ Remembered = %v, want 1 target", got)
	}
	got[0] = state.Target{Obj: 999}
	if ctx.Remembered[0] != origRemembered {
		t.Fatalf("mutating the Defined() result changed Ctx.Remembered to %v", ctx.Remembered)
	}

	// A nil Targets must still come back nil, not a spurious allocation, on
	// the no-Defined$-but-ValidTgts$ path that defers to copyTargets.
	if got := Defined(h, &Ctx{}, sa(t, "SP$ X | ValidTgts$ Creature | NumDmg$ 1")); got != nil {
		t.Fatalf("Defined() on nil Targets = %#v, want nil", got)
	}
}

func TestDefinedOpponentSkipsEliminatedSeats(t *testing.T) {
	h := newHost(t, 4)
	h.g.Players[2].Lost = true
	got := Defined(h, &Ctx{Controller: 1}, sa(t, "SP$ X | Defined$ Opponent"))
	if len(got) != 2 || got[0].Player != 3 || got[1].Player != 0 {
		t.Fatalf("Defined$ Opponent = %v", got)
	}
}

func TestDefinedDefaultsToSelfWithoutTargets(t *testing.T) {
	h, c := fixtureHost(t) // seat 0 controls object 1
	got := Defined(h, c, &cards.SA{Params: map[string]string{}})
	if len(got) != 1 || got[0].Obj != c.Source {
		t.Fatalf("no ValidTgts, no Defined: %v, want Self", got)
	}
	c.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	got = Defined(h, c, &cards.SA{Params: map[string]string{"ValidTgts": "Player"}})
	if len(got) != 1 || !got[0].IsPlayer {
		t.Fatalf("ValidTgts present: %v, want the chosen targets", got)
	}
	got = Defined(h, c, &cards.SA{Params: map[string]string{}})
	if len(got) != 1 || got[0].Obj != c.Source {
		t.Fatalf("a sub-ability without ValidTgts acts on Self even when the root had targets: %v", got)
	}
}

func TestDefinedTriggeredForms(t *testing.T) {
	h, c := fixtureHost(t)
	c.Remembered = []state.Target{{Obj: 2}, {Player: 1, IsPlayer: true}}
	for _, form := range []string{"TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy", "TriggeredSpellAbility", "TriggeredAttacker"} {
		got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": form}})
		if len(got) != 1 || got[0].Obj != 2 {
			t.Errorf("%s: %v", form, got)
		}
	}
	for _, form := range []string{"TriggeredDefendingPlayer", "TriggeredPlayer"} {
		got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": form}})
		if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
			t.Errorf("%s: %v", form, got)
		}
	}
	got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "TriggeredCardController"}})
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != h.Game().Obj(2).Controller {
		t.Errorf("TriggeredCardController: %v", got)
	}
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Parent"}}); len(got) != 1 || got[0].Obj != c.Source {
		t.Errorf("Parent: %v", got)
	}
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Equipped"}}); len(got) != 0 {
		t.Errorf("Equipped with nothing attached: %v", got)
	}
	h.Game().Obj(c.Source).AttachedTo = 2
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Equipped"}}); len(got) != 1 || got[0].Obj != 2 {
		t.Errorf("Equipped: %v", got)
	}
}

func TestSupportedListsRegisteredAPIs(t *testing.T) {
	Register("TestZ", func(Host, *Ctx, *cards.SA) {})
	t.Cleanup(func() { unregister("TestZ") })
	if !Supported()["api:TestZ"] {
		t.Fatal("Supported did not list a registered API")
	}
	if Supported()["api:NotRegistered"] {
		t.Fatal("Supported listed an API that was never registered")
	}
}

// TestRegistryConcurrentRegisterAndReadDoesNotRace guards the process-global
// registry against the case Register's own doc comment advertises as
// supported: registering (or re-registering, e.g. M3's plugin tier
// overriding a native primitive) after init time, concurrently with the
// engine's per-match goroutines calling Supported()/Resolve(). Run with
// `go test -race`, this failed against the plain, unsynchronized map that
// registry.go used before the copy-on-write atomicMap fix — see the task-15
// fix report for the before/after -race output.
func TestRegistryConcurrentRegisterAndReadDoesNotRace(t *testing.T) {
	Register("TestRace", func(Host, *Ctx, *cards.SA) {})
	t.Cleanup(func() { unregister("TestRace") })
	line := sa(t, "SP$ TestRace | NumDmg$ 1")

	const iterations = 500
	var wg sync.WaitGroup

	// One writer keeps re-registering the same API...
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			Register("TestRace", func(Host, *Ctx, *cards.SA) {})
		}
	}()

	// ...while several readers hammer the two read paths. Each reader gets
	// its own fakeHost so any data race the detector reports is in the
	// registry under test, not in this test's own event-log slice.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := newHost(t, 2)
			for i := 0; i < iterations; i++ {
				_ = Supported()
				Resolve(h, &Ctx{}, line)
			}
		}()
	}
	wg.Wait()
}
