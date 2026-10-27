package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const cantCrewVehicleSrc = "Name:Cant Crew Skiff\nManaCost:1\nTypes:Artifact Vehicle\nPT:2/2\nK:Crew:1\nOracle:x\n"

// cantCrewFixture seats the Crew 1 fixture Vehicle, the named real corpus
// Aura and `bears` real Grizzly Bears on seat 0's battlefield, attaches the
// Aura to the first bear (the "arrested" creature) and returns after a fresh
// priority round, so the crew ability's offer is current.
func cantCrewFixture(t *testing.T, auraName string, bears int, extras ...string) (e *Engine, cfg Config, vehicle, aura state.ObjID, creatures []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := []*cards.Card{card(t, cantCrewVehicleSrc), lookup(t, reg, auraName)}
	for _, src := range extras {
		deck = append(deck, card(t, src))
	}
	for range bears {
		deck = append(deck, lookup(t, reg, "Grizzly Bears"))
	}
	e, cfg = corpusEngineCfg(t, reg, deck, nil)
	vehicle = moveByName(t, e, 0, "Cant Crew Skiff", state.ZBattlefield)
	for range bears {
		creatures = append(creatures, moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield))
	}
	aura = moveByName(t, e, 0, auraName, state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{creatures[0]}})
	e.pending = nil
	e.priorityRound()
	if got := e.G.Obj(aura); got.Zone != state.ZBattlefield || got.AttachedTo != creatures[0] {
		t.Fatalf("precondition: %s zone %s attached to %d, want the battlefield on %d", auraName, got.Zone, got.AttachedTo, creatures[0])
	}
	if o := e.G.Obj(vehicle); o.Zone != state.ZBattlefield || e.IsCreature(vehicle) {
		t.Fatalf("precondition: vehicle zone %s creature %v", o.Zone, e.IsCreature(vehicle))
	}
	return
}

func TestCantCrewArrestedCreatureCannotCrew(t *testing.T) {
	t.Parallel()
	e, cfg, vehicle, _, bears := cantCrewFixture(t, "Intercessor's Arrest", 2)
	arrested, free := bears[0], bears[1]
	if !e.cantCrew(arrested) || e.cantCrew(free) || e.Power(arrested) < 1 || e.Power(free) < 1 {
		t.Fatalf("precondition: cantCrew arrested=%v free=%v, powers %d/%d", e.cantCrew(arrested), e.cantCrew(free), e.Power(arrested), e.Power(free))
	}

	// Only the arrested creature could pay Crew 1 if it were not barred:
	// tap the free one first so the Vehicle has no legal crew.
	e.emit(events.Event{Kind: events.Tap, Obj: free})
	e.pending = nil
	e.priorityRound()
	if _, offered := findAbilityOption(e, vehicle, 0); offered {
		t.Fatal("Crew offered with the only untapped creature under Intercessor's Arrest")
	}

	// Untap the free creature: Crew is offered and the election lists only it.
	e.emit(events.Event{Kind: events.Untap, Obj: free})
	e.pending = nil
	e.priorityRound()
	opt := abilityOption(t, e, vehicle, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("pending %+v, want the crew tap election", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != free {
		t.Fatalf("crew election options %+v, want only the un-enchanted creature %d (arrested %d)", d.Options, free, arrested)
	}
	replayCheck(t, e, cfg)
}

func TestCantCrewDoesNotBlockOtherTapCosts(t *testing.T) {
	t.Parallel()
	const trawler = "Name:Tap Engine\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ tapXType<1/Creature> | Defined$ You | LifeAmount$ 1 | SpellDescription$ Tap a creature you control: gain 1 life.\nOracle:x\n"
	e, cfg, _, _, bears := cantCrewFixture(t, "Intercessor's Arrest", 1, trawler)
	arrested := bears[0]
	engine := moveByName(t, e, 0, "Tap Engine", state.ZBattlefield)
	if !e.cantCrew(arrested) {
		t.Fatal("precondition: the creature is not barred from crewing")
	}
	e.pending = nil
	e.priorityRound()
	opt := abilityOption(t, e, engine, 0)
	submitChoices(t, e, opt.Index)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		if len(d.Options) != 1 || d.Options[0].Obj != arrested {
			t.Fatalf("tap election options %+v, want the arrested creature %d", d.Options, arrested)
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	passUntilStackEmpty(t, e, 20)
	if !e.G.Obj(arrested).Tapped {
		t.Fatal("the arrested creature did not pay a non-crew tap cost")
	}
	replayCheck(t, e, cfg)
}

func TestCantCrewRevokePrivilegesEndsWithAura(t *testing.T) {
	t.Parallel()
	e, cfg, vehicle, aura, bears := cantCrewFixture(t, "Revoke Privileges", 1)
	if !e.cantCrew(bears[0]) {
		t.Fatal("precondition: Revoke Privileges does not bar the creature from crewing")
	}
	if _, offered := findAbilityOption(e, vehicle, 0); offered {
		t.Fatal("Crew offered with the only creature under Revoke Privileges")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: aura, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	if e.cantCrew(bears[0]) {
		t.Fatal("the creature still cannot crew after the Aura left the battlefield")
	}
	abilityOption(t, e, vehicle, 0)
	replayCheck(t, e, cfg)
}

func TestParamCensusCompoundValidSAIsDescriptive(t *testing.T) {
	t.Parallel()
	if !ignoredParam("stat:CantAttack", "ValidSA") || !ignoredParam("stat:CantBlock", "ValidSA") || !ignoredParam("stat:CantCrew", "ValidSA") {
		t.Fatal("CantAttack/CantBlock/CantCrew ValidSA must be classified descriptive on a compound line")
	}
	if ignoredParam("stat:CantBeActivated", "ValidSA") {
		t.Fatal("CantBeActivated reads ValidSA; it must stay measurable")
	}
}
