package rules

// "Whenever this Vehicle becomes crewed" (CR 702.122e).
//
// A crew ability's resolution is the event the mode names: the engine's
// events.AbilityResolved marker carries an AbilityResolvedCrew flag, and
// Mode$ BecomesCrewed matches that flag against ValidCard$ (the Vehicle).
// FirstTimeCrewed$ is the per-object, per-turn latch; ValidCrewAmount$ counts
// the creatures that crewed THIS activation. A countered crew ability never
// emits the marker, so it never triggers.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// crewThisActivation activates vehicle's crew ability and elects exactly
// crewers. It leaves the crew ability on the stack; the caller drains it.
func crewThisActivation(t *testing.T, e *Engine, vehicle state.ObjID, crewers ...state.ObjID) {
	t.Helper()
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, vehicle, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("pending decision %+v, want the crew tap election (KChoose)", d)
	}
	var choices []int
	for _, o := range d.Options {
		for _, want := range crewers {
			if o.Obj == want {
				choices = append(choices, o.Index)
			}
		}
	}
	if len(choices) != len(crewers) {
		t.Fatalf("crew election offered %d of the %d intended crewers: %+v", len(choices), len(crewers), d.Options)
	}
	submitChoices(t, e, choices...)
}

// drainCrewTrigger drains the stack, answering a target ask with target when
// one offers it and with the first option otherwise.
func drainCrewTrigger(t *testing.T, e *Engine, target state.ObjID) {
	t.Helper()
	drainSetMechanic(t, e, func(d *decision.Decision) []int {
		if d.Kind == decision.KTarget {
			for _, o := range d.Options {
				if o.Obj == target {
					return []int{o.Index}
				}
			}
			t.Fatalf("target ask did not offer %d: %+v", target, d.Options)
		}
		return firstOption(d)
	})
}

// crewedTriggers counts the triggered abilities pushed whose source is id.
func crewedTriggers(e *Engine, id state.ObjID) int {
	return countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.TriggerPush && ev.Obj == id
	})
}

// TestMobilizerMechTriggersWhenCrewResolves proves the basic mode: a real
// Mobilizer Mech crewed by two creatures resolves its crew ability and its
// BecomesCrewed trigger animates another Vehicle.
func TestMobilizerMechTriggersWhenCrewResolves(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t)
	mob := addToBattlefield(t, e, lookup(t, reg, "Mobilizer Mech"), 0)
	other := addToBattlefield(t, e, lookup(t, reg, "Unicycle"), 0)
	b1 := addToBattlefield(t, e, lookup(t, reg, "Grizzly Bears"), 0)
	b2 := addToBattlefield(t, e, lookup(t, reg, "Grizzly Bears"), 0)
	for _, id := range []state.ObjID{mob, other, b1, b2} {
		e.G.Obj(id).SummonSick = false
	}
	// Preconditions the assertions depend on: the target is a Vehicle that
	// is not yet a creature, and the two bears reach Crew 3.
	if !e.G.Obj(other).Face().HasTypeWord("Vehicle") || e.IsCreature(other) {
		t.Fatalf("precondition: Unicycle is not a noncreature Vehicle: %+v", e.G.Obj(other))
	}
	if p := e.Power(b1) + e.Power(b2); p < 3 {
		t.Fatalf("precondition: the two bears' power is %d, want >= 3", p)
	}
	if n := crewedTriggers(e, mob); n != 0 {
		t.Fatalf("precondition: Mobilizer Mech already has %d triggers", n)
	}

	crewThisActivation(t, e, mob, b1, b2)
	if !e.G.Obj(b1).Tapped || !e.G.Obj(b2).Tapped {
		t.Fatal("precondition: the crew cost did not tap the elected creatures")
	}
	drainCrewTrigger(t, e, other)

	if !e.IsCreature(other) {
		t.Fatalf("Mode$ BecomesCrewed never fired: the other Vehicle is still not a creature: %+v", e.G.Obj(other))
	}
	if n := crewedTriggers(e, mob); n != 1 {
		t.Fatalf("Mobilizer Mech pushed %d triggers for one crew resolution, want 1", n)
	}
}

// TestBecomesCrewedNotOnCounteredCrew proves a countered crew ability does not
// trigger the mode: the crew cost was paid (the Crew markers exist and the
// creatures are tapped), but the ability never resolved, so no
// AbilityResolved marker and no BecomesCrewed trigger.
func TestBecomesCrewedNotOnCounteredCrew(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mob := lookup(t, reg, "Mobilizer Mech")
	other := lookup(t, reg, "Unicycle")
	bear := lookup(t, reg, "Grizzly Bears")
	elf := lookup(t, reg, "Llanowar Elves")
	stifle := lookup(t, reg, "Stifle")
	e := counterHands(t, nil, []*cards.Card{stifle},
		[]*cards.Card{mob, other, bear, elf}, nil)
	mobID := battlefieldID(t, e, "Mobilizer Mech")
	otherID := battlefieldID(t, e, "Unicycle")
	bearID := battlefieldID(t, e, "Grizzly Bears")
	elfID := battlefieldID(t, e, "Llanowar Elves")
	stifleID := handObj(t, e, 1, "Stifle")
	e.G.Players[1].Pool[state.MU] = 2
	e.priorityRound()

	crewThisActivation(t, e, mobID, bearID, elfID)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: the crew ability is not alone on the stack: %+v", e.G.Stack)
	}
	crewStack := e.G.Stack[0]
	if o := e.G.Obj(crewStack); o == nil || o.Source != mobID {
		t.Fatalf("precondition: the stack object is not Mobilizer Mech's crew ability: %+v", o)
	}

	// Seat 0 passes; seat 1 casts Stifle at the crew ability.
	passOnce(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected seat 1's priority, got %+v", d)
	}
	idx := castOptionIdx(d, "cast", stifleID)
	if idx < 0 {
		t.Fatalf("Stifle is not offered against the crew ability: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Stifle's target ask, got %+v", d)
	}
	target := -1
	for _, o := range d.Options {
		if o.Obj == crewStack {
			target = o.Index
		}
	}
	if target < 0 {
		t.Fatalf("Stifle cannot target the crew ability: %+v", d.Options)
	}
	submitChoices(t, e, target)
	drainSetMechanic(t, e, firstOption)

	if !e.G.Obj(bearID).Tapped || !e.G.Obj(elfID).Tapped {
		t.Fatal("precondition: the crew cost was not paid before Stifle resolved")
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Crew && len(ev.IDs) > 0 && ev.IDs[0] == mobID
	}); got == 0 {
		t.Fatal("precondition: no Crew markers were recorded for the countered activation")
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.AbilityResolved && ev.Obj == mobID
	}); got != 0 {
		t.Fatalf("a countered crew ability emitted %d AbilityResolved markers, want 0", got)
	}
	if e.IsCreature(otherID) {
		t.Fatal("Mode$ BecomesCrewed fired after its crew ability was countered")
	}
	if got := crewedTriggers(e, mobID); got != 0 {
		t.Fatalf("Mobilizer Mech pushed %d triggers for a countered crew ability, want 0", got)
	}
}

// TestMindlinkMechFirstTimeEachTurn proves FirstTimeCrewed$: two crew
// resolutions of the same object in one turn push exactly one trigger.
func TestMindlinkMechFirstTimeEachTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t)
	mind := addToBattlefield(t, e, lookup(t, reg, "Mindlink Mech"), 0)
	elf1 := addToBattlefield(t, e, lookup(t, reg, "Llanowar Elves"), 0)
	elf2 := addToBattlefield(t, e, lookup(t, reg, "Llanowar Elves"), 0)
	for _, id := range []state.ObjID{mind, elf1, elf2} {
		e.G.Obj(id).SummonSick = false
	}
	if e.Power(elf1) != 1 || e.Power(elf2) != 1 {
		t.Fatalf("precondition: the elves are %d/%d, want 1/1 to pay Crew 1", e.Power(elf1), e.Power(elf2))
	}
	if e.IsCreature(mind) {
		t.Fatal("precondition: Mindlink Mech is already a creature")
	}

	// Two crew activations in response: both costs are paid before either
	// resolves, so both Crew marker blocks precede both resolutions.
	crewThisActivation(t, e, mind, elf1)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: first crew ability not on the stack: %+v", e.G.Stack)
	}
	crewThisActivation(t, e, mind, elf2)
	if len(e.G.Stack) != 2 {
		t.Fatalf("precondition: second crew ability not on the stack: %+v", e.G.Stack)
	}
	drainCrewTrigger(t, e, elf1)

	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Crew && len(ev.IDs) > 0 && ev.IDs[0] == mind
	}); got != 2 {
		t.Fatalf("precondition: %d Crew markers for two Crew 1 activations, want 2", got)
	}
	if got := crewedTriggers(e, mind); got != 1 {
		t.Fatalf("FirstTimeCrewed$ pushed %d triggers for two crew resolutions, want 1", got)
	}
}

// TestMightyServantCrewedByExactlyTwo proves ValidCrewAmount$: "if it was
// crewed by exactly two creatures". Three crewers do not trigger; two do.
// Each scenario is a fresh object, so FirstTimeCrewed$ is true in both.
func TestMightyServantCrewedByExactlyTwo(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	crewCount := func(t *testing.T, names ...string) int {
		t.Helper()
		e := handEngine(t)
		servant := addToBattlefield(t, e, lookup(t, reg, "Mighty Servant of Leuk-o"), 0)
		e.G.Obj(servant).SummonSick = false
		var crewers []state.ObjID
		for _, name := range names {
			id := addToBattlefield(t, e, lookup(t, reg, name), 0)
			e.G.Obj(id).SummonSick = false
			crewers = append(crewers, id)
		}
		power := 0
		for _, id := range crewers {
			power += int(e.Power(id))
		}
		if power < 4 {
			t.Fatalf("precondition: the intended crewers' total power is %d, want >= 4 (Crew 4)", power)
		}
		crewThisActivation(t, e, servant, crewers...)
		drainCrewTrigger(t, e, 0)
		if got := countEvents(e, func(ev events.Event) bool {
			return ev.Kind == events.Crew && len(ev.IDs) > 0 && ev.IDs[0] == servant
		}); got != len(crewers) {
			t.Fatalf("precondition: %d Crew markers for %d crewers", got, len(crewers))
		}
		return crewedTriggers(e, servant)
	}

	t.Run("three crewers do not trigger", func(t *testing.T) {
		if got := crewCount(t, "Grizzly Bears", "Grizzly Bears", "Llanowar Elves"); got != 0 {
			t.Fatalf("Mighty Servant pushed %d triggers when crewed by three creatures, want 0", got)
		}
	})
	t.Run("two crewers trigger", func(t *testing.T) {
		if got := crewCount(t, "Grizzly Bears", "Grizzly Bears"); got != 1 {
			t.Fatalf("Mighty Servant pushed %d triggers when crewed by exactly two creatures, want 1", got)
		}
	})
}

// TestEveryBecomesCrewedCardIsPlayable is the class census: implementing
// trig:BecomesCrewed leaves these four cards fully supported.
func TestEveryBecomesCrewedCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Mindlink Mech", "Mobilizer Mech", "Ghost Ark", "Mighty Servant of Leuk-o",
	} {
		c := lookup(t, reg, name)
		if missing := reg.Unsupported(c, effects.Supported()); len(missing) != 0 {
			t.Errorf("%s is still unsupported: %v", name, missing)
		}
	}
}
