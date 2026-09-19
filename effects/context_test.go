package effects

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
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
	controls   []ControlGrant
	n          int
	dmgSrc     state.ObjID
	batch      []state.ObjID
	// castFromHand is the WasCastFromHandByYou answer the double reports;
	// the eval-level Count$wasCastFromYourHandByYou tests flip it to pin the
	// true branch (the real log-scan read is pinned in rules).
	castFromHand bool
	// typeChoices is the TypeChoices answer the double reports (nil by
	// default): the effects-side ChooseType tests configure it to pose a
	// real option list. Nil routes ChooseType through AskEmpty — the
	// unchanged deterministic fallback.
	typeChoices []decision.Option
}

func (h *fakeHost) Game() *state.Game { return h.g }
func (h *fakeHost) Emit(e events.Event) {
	h.log = append(h.log, e)
	events.Apply(h.g, e)
}

// EmitTap has no trigger matcher to hand the tapper and entry provenance to,
// so the double records the same plain Tap event the engine logs.
func (h *fakeHost) EmitTap(obj state.ObjID, _ state.PlayerID, _ bool) {
	h.Emit(events.Event{Kind: events.Tap, Obj: obj})
}
func (h *fakeHost) Rand(n int) int { h.n++; return 0 }
func (h *fakeHost) ShuffleLibrary(_ state.PlayerID, order []state.ObjID) []state.ObjID {
	out := append([]state.ObjID(nil), order...)
	for i := len(out) - 1; i > 0; i-- {
		j := h.Rand(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}
func (h *fakeHost) AddContinuous(ce state.ContinuousEffect) {
	h.continuous = append(h.continuous, ce)
}

// ContinuousNamed scans the double's own recorded slice: the effects tests
// have no engine registry to ask.
func (h *fakeHost) ContinuousNamed(controller state.PlayerID, name string) bool {
	for _, ce := range h.continuous {
		if ce.Controller == controller && ce.Name == name {
			return true
		}
	}
	return false
}
func (h *fakeHost) RegisterControl(gr ControlGrant) {
	h.controls = append(h.controls, gr)
}
func (h *fakeHost) LegalTargets(chooser state.PlayerID, source state.ObjID, sa *cards.SA) []state.Target {
	var out []state.Target
	spec := sa.Params["ValidTgts"]
	if strings.Contains(spec, "Player") || strings.Contains(spec, "Opponent") || strings.Contains(spec, "Any") {
		for _, p := range h.g.AliveFrom(chooser) {
			if MatchesPlayerSpec(h.g, spec, p, chooser) || spec == "Any" {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
	}
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.ID != source && o.Zone == state.ZBattlefield && MatchesObjectCtx(h.g, spec, o, SpecContext{You: chooser, Source: source}) {
			out = append(out, state.Target{Obj: o.ID})
		}
	}
	return out
}

// RegenerationDisallowed has no registry to consult here (the engine-side
// restriction lives in rules.Engine); the effects-package tests that exercise
// ReplaceDestruction set up their own boards and never rely on an
// Effect-registered CantRegenerate. Reporting false keeps the double honest
// rather than inventing a registry it cannot answer for.
func (h *fakeHost) RegenerationDisallowed(id state.ObjID) bool { return false }

// The damage-batch bracket has nothing to latch here (no trigger machinery),
// so the double reports no-ops; the dealDamage loops' bracketing still runs.
func (h *fakeHost) BeginDamageBatch() {}
func (h *fakeHost) EndDamageBatch()   {}

// CastThisTurn has no real turn log to count here (Task 17); the effects
// package tests set up their own boards, so the double reports zero.
func (h *fakeHost) CastThisTurn() int { return 0 }

// LifeLostThisTurn has no event log here; the double reports zero (the same
// conservative no-op as CastThisTurn).
func (h *fakeHost) LifeLostThisTurn(_ state.PlayerID) int32 { return 0 }

// LifeGainedThisTurn has no event log here; the double reports zero (the
// same conservative no-op as LifeLostThisTurn).
func (h *fakeHost) LifeGainedThisTurn(_ state.PlayerID) int32 { return 0 }

// TurnsTaken has no event log here; the double reports zero.
func (h *fakeHost) TurnsTaken(_ state.PlayerID) int32 { return 0 }

// SpellsCastThisTurnMatching has no event log here; the double reports zero.
func (h *fakeHost) SpellsCastThisTurnMatching(_ state.PlayerID, _ string) int { return 0 }

// SpellsCastThisTurnMatchingExcluding has no event log here; the double
// reports zero (the same conservative no-op as SpellsCastThisTurnMatching).
func (h *fakeHost) SpellsCastThisTurnMatchingExcluding(_ state.PlayerID, _ string, _ state.ObjID) int {
	return 0
}

// WasCastFromHandByYou has no cast log here; the double reports false (the
// same conservative no-op as CastThisTurn), so the Count$
// wasCastFromYourHandByYou branch head's fakeHost evals take the ifFalse
// branch; the true branch is pinned end to end on the real engine in rules
// (the Myojin cycle's corpus tests).
func (h *fakeHost) WasCastFromHandByYou(_ state.ObjID, _ state.PlayerID) bool { return h.castFromHand }

// CommanderIdentityColourCount has no commander bookkeeping here; the double
// reports zero (the same replay-derivable class as TurnsTaken above).
func (h *fakeHost) CommanderIdentityColourCount(_ state.PlayerID) int { return 0 }

// AttackersThisTurn has no combat log here; the double reports zero (the same
// conservative no-op as CastThisTurn).
func (h *fakeHost) AttackersThisTurn() int { return 0 }

// HasKeyword has no layer system to consult here (see the type doc comment),
// so it reads the printed face directly -- enough for the effects-package
// tests, which set up Indestructible by mutating Card.Faces[0].Keywords.
func (h *fakeHost) HasKeyword(id state.ObjID, kw string) bool {
	o := h.g.Obj(id)
	return o != nil && o.Face() != nil && o.Face().HasKeyword(kw)
}

// UmbraArmorAura has no layer system to consult here either (Umbra Mystic's
// grant is a rules-side derived keyword); the double reports none, so the
// effects-package tests that drive ReplaceUmbraArmor directly must seed a
// printed keyword on the Aura's face.
func (h *fakeHost) UmbraArmorAura(_ state.ObjID) state.ObjID { return 0 }
func (h *fakeHost) Power(id state.ObjID) int32 {
	o := h.g.Obj(id)
	if o == nil || o.Face() == nil {
		return 0
	}
	return int32(o.Face().Power()) + o.Counter("P1P1") - o.Counter("M1M1")
}
func (h *fakeHost) Toughness(id state.ObjID) int32 {
	o := h.g.Obj(id)
	if o == nil || o.Face() == nil {
		return 0
	}
	return int32(o.Face().Toughness()) + o.Counter("P1P1") - o.Counter("M1M1")
}
func (h *fakeHost) IsCreature(id state.ObjID) bool {
	o := h.g.Obj(id)
	return o != nil && o.Face() != nil && o.Face().IsCreature()
}

// Ask reports false: an effects-package test double has no engine to drive,
// so a mid-resolution ask falls back to the primitive's deterministic
// stand-in (effCharm's first mode, effCopySpellAbility's decline) -- which
// is exactly today's no-ask behaviour, now with the engines it is a fallback
// for clearly named (R-9).
func (h *fakeHost) Ask(d *decision.Decision) bool { return false }

// TypeChoices serves the double's configured typeChoices list (nil by
// default): nil routes ChooseType through AskEmpty — the unchanged
// deterministic fallback — so the existing fallback pins pass untouched.
func (h *fakeHost) TypeChoices(_ state.PlayerID, _ string) []decision.Option {
	return h.typeChoices
}

// Suspended reports false: an effects-package test double never actually
// suspends a resolution (its Ask always returns false, so the asking effect
// falls back to its deterministic stand-in and the chain — if it had a
// SubAbility — finishes in one pass). This keeps effects.Resolve's
// suspended-check from breaking the chain on a host that never asked; the
// real suspension behaviour is exercised through the rules engine, where
// Engine.Suspended reports e.resume != nil.
func (h *fakeHost) Suspended() bool { return false }

// SuspendContinuation is a no-op: an effects-package test double never
// suspends (its Ask returns false), so effects.Resolve never reaches the
// suspended branch that would call it. Kept to satisfy the Host interface.
func (h *fakeHost) SuspendContinuation(*cards.SA) {}

// SuspendUnless is a no-op for the same reason as SuspendContinuation.
func (h *fakeHost) SuspendUnless(*cards.SA, bool) {}

func (h *fakeHost) ReplaceEvent(string, string, int32) {}

func (h *fakeHost) EmitDamage(e events.Event) events.Event {
	h.Emit(e)
	return e
}
func (h *fakeHost) CounterAllowed(state.ObjID, state.ObjID) bool { return true }

// SuspendRepeat is a no-op for the same reason as SuspendContinuation.
func (h *fakeHost) SuspendRepeat(RepeatSuspension) {}

// SuspendCharmRest is a no-op for the same reason as SuspendContinuation.
func (h *fakeHost) SuspendCharmRest(*cards.SA, []string) {}

// SetDamageSource records the published damage source on the double (the
// last value wins) and returns the previous one, mirroring the engine's
// set-and-restore contract so an emitter's restore is observable.
func (h *fakeHost) SetDamageSource(id state.ObjID) state.ObjID {
	prev := h.dmgSrc
	h.dmgSrc = id
	return prev
}

// BatchDepartures is a no-op snapshot: an effects-package double has no
// engine-side departure capture to feed, so it keeps only the fact a batch
// was declared (never asserted on today; the rules package owns the
// behaviour this method exists for).
func (h *fakeHost) BatchDepartures(ids []state.ObjID) { h.batch = ids }
func (h *fakeHost) EndBatchDepartures()               { h.batch = nil }

func newHost(t testing.TB, seats int) *fakeHost {
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

func sa(t testing.TB, line string) *cards.SA {
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

func TestCompiledAPIRegistryDispatchParity(t *testing.T) {
	defer Register("Draw", effDraw)
	defer unregister("TestCompiledUnknown")

	bound := &cards.SA{Kind: "SP", API: "Draw"}
	r := cards.NewRegistry()
	r.Add(&cards.Card{Faces: []*cards.Face{{Abilities: []*cards.SA{bound}}}})
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	if bound.CompiledAPI() != cards.APIDraw {
		t.Fatalf("compiled API = %d, want Draw", bound.CompiledAPI())
	}

	var first, second, unknown int
	Register("Draw", func(Host, *Ctx, *cards.SA) { first++ })
	snapshot := registry.load()
	if snapshot.byName["Draw"] == nil || snapshot.byCode[cards.APIDraw] == nil {
		t.Fatal("Draw registration was not published in both lookup views")
	}
	Resolve(newHost(t, 2), &Ctx{}, bound)
	Resolve(newHost(t, 2), &Ctx{}, &cards.SA{Kind: "SP", API: "Draw"})
	if first != 2 {
		t.Fatalf("first Draw implementation ran %d times, want bound and textual dispatch", first)
	}

	Register("Draw", func(Host, *Ctx, *cards.SA) { second++ })
	Resolve(newHost(t, 2), &Ctx{}, bound)
	Resolve(newHost(t, 2), &Ctx{}, &cards.SA{Kind: "SP", API: "Draw"})
	if second != 2 || first != 2 {
		t.Fatalf("replacement dispatch: first=%d second=%d, want 2/2", first, second)
	}

	Register("TestCompiledUnknown", func(Host, *Ctx, *cards.SA) { unknown++ })
	Resolve(newHost(t, 2), &Ctx{}, &cards.SA{Kind: "SP", API: "TestCompiledUnknown"})
	if unknown != 1 {
		t.Fatalf("registered unknown API ran %d times, want 1", unknown)
	}

	unregister("Draw", "TestCompiledUnknown")
	snapshot = registry.load()
	if snapshot.byName["Draw"] != nil || snapshot.byCode[cards.APIDraw] != nil {
		t.Fatal("Draw unregistration was not published in both lookup views")
	}
	h := newHost(t, 2)
	Resolve(h, &Ctx{}, bound)
	if len(h.log) != 1 || h.log[0].Text != "unimplemented API Draw" {
		t.Fatalf("unregistered compiled API log = %+v", h.log)
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
