package rules

// CR 702.122e: "Whenever [this Vehicle] becomes crewed" means "whenever a crew
// ability of [this Vehicle] resolves". These tests pin that timing: the
// events.CrewAction marker Mode$ BecomesCrewed matches is emitted by the crew
// ability's resolution, never by its activation, so a crew ability that is
// countered (or whose Vehicle left the battlefield in response) emits nothing
// and the trigger never fires.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// crewMechToTheStack activates Mobilizer Mech's Crew 3 with the Bears and the
// Elves and leaves the crew ability ON the stack, unresolved, returning the
// crew ability's stack object. It asserts the preconditions the counter
// scenarios lean on: the crewers tapped (the per-crewer Crew markers exist),
// the ability is the only stack object, no CrewAction marker exists yet, and
// the Mech is not yet a creature.
func crewMechToTheStack(t *testing.T) (e *Engine, mech, quinjet state.ObjID, ability state.ObjID) {
	t.Helper()
	e, quinjet, ids := crewedVehicleFixture(t, "Mobilizer Mech", "Grizzly Bears", "Llanowar Elves")
	mech, bears, elves := ids[0], ids[1], ids[2]
	submitChoices(t, e, abilityOption(t, e, mech, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("crew tap election %+v, want KChoose", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexChoice(t, d, bears), indexChoice(t, d, elves)}}); err != nil {
		t.Fatalf("legal two-crewer election rejected: %v", err)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack after paying Crew 3 = %v, want exactly the crew ability", e.G.Stack)
	}
	ability = e.G.Stack[0]
	crewers := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Crew && len(ev.IDs) == 1 && ev.IDs[0] == mech {
			crewers++
		}
	}
	if crewers != 2 || !e.G.Obj(bears).Tapped || !e.G.Obj(elves).Tapped {
		t.Fatalf("precondition: %d per-crewer Crew markers, bears tapped %v elves tapped %v; want the cost paid by two tapped creatures",
			crewers, e.G.Obj(bears).Tapped, e.G.Obj(elves).Tapped)
	}
	if got := len(crewActionMarkers(e, mech)); got != 0 {
		t.Fatalf("CrewAction markers while the crew ability is still on the stack = %d, want 0 (the Vehicle has not become crewed)", got)
	}
	if e.IsCreature(mech) {
		t.Fatal("precondition: the Mech is already a creature with its crew ability unresolved")
	}
	return e, mech, quinjet, ability
}

// assertNoBecomesCrewed fails when the Mech's crew resolved into a crewed
// event: a marker, a queued/stacked trigger, a trigger target ask, or the
// Quinjet's animation.
func assertNoBecomesCrewed(t *testing.T, e *Engine, mech, quinjet state.ObjID) {
	t.Helper()
	if got := crewActionMarkers(e, mech); len(got) != 0 {
		t.Fatalf("CrewAction markers %+v, want none (the crew ability never resolved)", got)
	}
	if len(e.G.Stack) != 0 || len(e.pendingTriggers) != 0 {
		t.Fatalf("stack %v / pending triggers %d, want both empty", e.G.Stack, len(e.pendingTriggers))
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget && strings.Contains(d.Prompt, "Mobilizer Mech") {
		t.Fatalf("Mobilizer Mech's BecomesCrewed trigger asked for a target: %+v", d)
	}
	if e.IsCreature(quinjet) {
		t.Fatal("the Quinjet was animated by a BecomesCrewed trigger that must not have fired")
	}
}

// TestEDHBecomesCrewedCounteredCrewAbilityDoesNotFire: the crew ability is
// countered (the Stifle shape: stack to exile, "countered") before it
// resolves. The Vehicle never became crewed, so no CrewAction marker, no
// trigger, no animation -- even though the crewers did tap as the cost.
func TestEDHBecomesCrewedCounteredCrewAbilityDoesNotFire(t *testing.T) {
	t.Parallel()
	e, mech, quinjet, ability := crewMechToTheStack(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: ability, From: state.ZStack, To: state.ZExile, Text: "countered"})
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the countered ability is still on the stack %v", e.G.Stack)
	}
	e.pending = nil
	e.priorityRound()
	assertNoBecomesCrewed(t, e, mech, quinjet)
	if e.IsCreature(mech) {
		t.Fatal("a countered crew ability animated the Mech")
	}
}

// TestEDHBecomesCrewedVehicleGoneInResponseDoesNotFire: the Vehicle leaves
// the battlefield with its crew ability on the stack. The ability resolves
// against a Vehicle that is no longer there (CR 400.7), so nothing became
// crewed: no marker, no trigger.
func TestEDHBecomesCrewedVehicleGoneInResponseDoesNotFire(t *testing.T) {
	t.Parallel()
	e, mech, quinjet, _ := crewMechToTheStack(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: mech, From: state.ZBattlefield, To: state.ZHand})
	if o := e.G.Obj(mech); o == nil || o.Zone == state.ZBattlefield {
		t.Fatalf("precondition: the Mech is still on the battlefield: %+v", o)
	}
	passUntilStackEmpty(t, e, 30)
	assertNoBecomesCrewed(t, e, mech, quinjet)
}

// TestEDHBecomesCrewedTriggerSeesTheVehicleAsACreature: the trigger is queued
// by the crew ability's resolution, so when it asks for its target the Mech
// IS an artifact creature already (CR 702.122e ordering).
func TestEDHBecomesCrewedTriggerSeesTheVehicleAsACreature(t *testing.T) {
	t.Parallel()
	e, mech, quinjet, _ := crewMechToTheStack(t)
	asked := false
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack %v)", e.G.Stack)
		}
		if d.Kind == decision.KTarget && strings.Contains(d.Prompt, "Mobilizer Mech") {
			asked = true
			if !e.IsCreature(mech) {
				t.Fatal("the BecomesCrewed trigger is resolving before the Vehicle became a creature (CR 702.122e)")
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexChoice(t, d, quinjet)}}); err != nil {
				t.Fatalf("legal trigger target rejected: %v", err)
			}
			continue
		}
		passOnceP(t, e)
	}
	if !asked {
		t.Fatal("the BecomesCrewed trigger never asked for its target after the crew ability resolved")
	}
	if got := crewActionMarkers(e, mech); len(got) != 1 || len(got[0].IDs) != 2 {
		t.Fatalf("CrewAction markers %+v, want one carrying both crewers", got)
	}
}

// TestEDHBecomesCrewedGhostArkGrantsUnearth plays Ghost Ark's printed text:
// crewing it makes "each artifact creature card in your graveyard gain unearth
// {3} until end of turn". Steel Overseer (an artifact creature card) sits in
// the graveyard; it gains Unearth when the crew ability resolves and not
// before.
func TestEDHBecomesCrewedGhostArkGrantsUnearth(t *testing.T) {
	t.Parallel()
	e, ark, ids := crewVehicleFixtureNamed(t, "Ghost Ark", "Steel Overseer", "Grizzly Bears")
	overseer, bears := ids[0], ids[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: overseer, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	if o := e.G.Obj(overseer); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Steel Overseer in %v, want graveyard", o.Zone)
	}
	if e.HasKeyword(overseer, "Unearth") {
		t.Fatal("precondition: Steel Overseer already has Unearth")
	}
	submitChoices(t, e, abilityOption(t, e, ark, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("crew tap election %+v, want KChoose", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexChoice(t, d, bears)}}); err != nil {
		t.Fatalf("crew election rejected: %v", err)
	}
	if e.HasKeyword(overseer, "Unearth") {
		t.Fatal("Unearth was granted before the crew ability resolved")
	}
	passUntilStackEmpty(t, e, 30)
	if len(crewActionMarkers(e, ark)) != 1 {
		t.Fatalf("Ghost Ark's crew action markers %+v, want exactly one", crewActionMarkers(e, ark))
	}
	if !e.HasKeyword(overseer, "Unearth") {
		t.Fatalf("Steel Overseer in the graveyard lacks Unearth after Ghost Ark became crewed (keywords %v)", e.Keywords(overseer))
	}
}

// crewVehicleFixtureNamed is crewedVehicleFixture without the Quinjet: the
// named Vehicle first, then the other named cards, all on seat 0's
// battlefield.
func crewVehicleFixtureNamed(t *testing.T, names ...string) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var extras []*cards.Card
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("corpus has no %s", n)
		}
		extras = append(extras, c)
	}
	e := corpusEngine(t, reg, extras, nil)
	var ids []state.ObjID
	for _, n := range names {
		ids = append(ids, moveByName(t, e, 0, n, state.ZBattlefield))
	}
	e.pending = nil
	e.priorityRound()
	return e, ids[0], ids[1:]
}
