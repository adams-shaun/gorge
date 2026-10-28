package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// attachFixture puts equip next to bearer on p0's battlefield and attaches it,
// asserting the attachment holds before the test's trigger event.
func attachFixture(t *testing.T, e *Engine, equip, bearer state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.Attach, Obj: equip, IDs: []state.ObjID{bearer}})
	e.checkStateBased()
	if got := e.G.Obj(equip).AttachedTo; got != bearer {
		t.Fatalf("precondition: attached to %d, want %d", got, bearer)
	}
}

func hasKeywordNamed(e *Engine, id state.ObjID, kw string) bool {
	for _, k := range e.Derived(id).Keywords {
		if k == kw {
			return true
		}
	}
	return false
}

// CR 704.5p / 301.5c: an Equipment that becomes a creature (March of the
// Machines) becomes unattached and stays on the battlefield.
func TestCR704_5pAnimatedEquipmentUnattaches(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Ornithopter", "Avacyn's Collar", "March of the Machines")
	orni := moveByName(t, e, 0, "Ornithopter", state.ZBattlefield)
	collar := moveByName(t, e, 0, "Avacyn's Collar", state.ZBattlefield)
	attachFixture(t, e, collar, orni)
	if e.IsCreature(collar) {
		t.Fatal("precondition: the Collar is already a creature")
	}
	if p := e.Derived(orni).Power; p != 1 || !hasKeywordNamed(e, orni, "Vigilance") {
		t.Fatalf("precondition: Ornithopter power %d vigilance %v, want 1 with vigilance", p, hasKeywordNamed(e, orni, "Vigilance"))
	}
	moveByName(t, e, 0, "March of the Machines", state.ZBattlefield)
	if !e.IsCreature(collar) {
		t.Fatal("precondition: March did not animate the Collar")
	}
	e.checkStateBased()
	co := e.G.Obj(collar)
	if co.Zone != state.ZBattlefield || co.AttachedTo != 0 {
		t.Fatalf("Collar zone %v attached to %d, want unattached on the battlefield", co.Zone, co.AttachedTo)
	}
	if d := e.Derived(collar); d.Power != 1 || d.Toughness != 1 {
		t.Fatalf("Collar is %d/%d, want 1/1", d.Power, d.Toughness)
	}
	if d := e.Derived(orni); d.Power != 0 || d.Toughness != 2 || hasKeywordNamed(e, orni, "Vigilance") {
		t.Fatalf("Ornithopter is %d/%d vigilance=%v, want 0/2 without vigilance", d.Power, d.Toughness, hasKeywordNamed(e, orni, "Vigilance"))
	}
}

// Relic's Roar on an attached Equipment makes it a 4/3 Dinosaur artifact
// creature, which unattaches.
func TestCR704_5pRelicsRoarOnAttachedEquipmentUnattaches(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Ornithopter", "Avacyn's Collar", "Relic's Roar")
	orni := moveByName(t, e, 0, "Ornithopter", state.ZBattlefield)
	collar := moveByName(t, e, 0, "Avacyn's Collar", state.ZBattlefield)
	attachFixture(t, e, collar, orni)
	roar := moveByName(t, e, 0, "Relic's Roar", state.ZHand)
	addMana(t, e, 0, "U")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == roar {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Relic's Roar: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	submitTarget(t, e, collar)
	passUntilStackEmpty(t, e, 20)
	if !e.IsCreature(collar) {
		t.Fatal("precondition: Relic's Roar did not animate the Collar")
	}
	e.checkStateBased()
	co := e.G.Obj(collar)
	if co.Zone != state.ZBattlefield || co.AttachedTo != 0 {
		t.Fatalf("Collar zone %v attached to %d, want unattached on the battlefield", co.Zone, co.AttachedTo)
	}
	if d := e.Derived(collar); d.Power != 4 || d.Toughness != 3 {
		t.Fatalf("Collar is %d/%d, want 4/3", d.Power, d.Toughness)
	}
}

// CR 303.4d: an Aura that becomes a creature is unattached, then put into its
// owner's graveyard. Mycosynth Lattice makes Pacifism an artifact and March of
// the Machines makes every noncreature artifact a creature.
func TestCR704_5pAnimatedAuraGoesToGraveyard(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Ornithopter", "Pacifism", "Mycosynth Lattice", "March of the Machines")
	orni := moveByName(t, e, 0, "Ornithopter", state.ZBattlefield)
	pac := moveByName(t, e, 0, "Pacifism", state.ZBattlefield)
	attachFixture(t, e, pac, orni)
	moveByName(t, e, 0, "Mycosynth Lattice", state.ZBattlefield)
	moveByName(t, e, 0, "March of the Machines", state.ZBattlefield)
	if !e.IsCreature(pac) {
		t.Fatal("precondition: Pacifism is not a creature")
	}
	e.checkStateBased()
	if z := e.G.Obj(pac).Zone; z != state.ZGraveyard {
		t.Fatalf("Pacifism zone %v, want the graveyard", z)
	}
	if !hasEvent(e, events.Unattached, pac) {
		t.Fatal("Pacifism went to the graveyard without an Unattached event first")
	}
}

// CR 702.151b: an attached Reconfigure Equipment is not a creature and stays
// attached.
func TestCR704_5pReconfigureStaysAttached(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Ornithopter", "Lion Sash")
	orni := moveByName(t, e, 0, "Ornithopter", state.ZBattlefield)
	sash := moveByName(t, e, 0, "Lion Sash", state.ZBattlefield)
	if !e.IsCreature(sash) {
		t.Fatal("precondition: an unattached Lion Sash is a creature")
	}
	e.emit(events.Event{Kind: events.Attach, Obj: sash, IDs: []state.ObjID{orni}})
	e.checkStateBased()
	e.checkStateBased()
	if got := e.G.Obj(sash).AttachedTo; got != orni {
		t.Fatalf("Lion Sash attached to %d after SBA, want %d", got, orni)
	}
	if e.IsCreature(sash) {
		t.Fatal("an attached Lion Sash is a creature")
	}
}
