package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAttachedReconfigureLosesCreatureSubtypes pins CR 702.151b with CR
// 205.1a on Acquisition Octopus ("Artifact Creature -- Equipment Octopus",
// K:Reconfigure:2), the SUP farm row where Forge (the corroborating engine)
// reads the attached Octopus as [artifact equipment] and gorge kept Octopus:
// while attached it is not a creature, so the Octopus creature subtype goes
// with the lost Creature card type; unattaching restores both.
func TestAttachedReconfigureLosesCreatureSubtypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := linkBoard(t, reg, []string{"Acquisition Octopus", "Grizzly Bears"}, nil)
	octo := findOnBoard(t, e, 0, "Acquisition Octopus")
	bear := findOnBoard(t, e, 0, "Grizzly Bears")

	// Preconditions: the unattached Octopus is a creature with the Octopus
	// subtype, so the attached assertions are about the switch.
	if types := e.Derived(octo).Types; !slices.Contains(types, "Creature") || !slices.Contains(types, "Octopus") {
		t.Fatalf("unattached Acquisition Octopus types %v, want Creature and Octopus", types)
	}
	if !effects.CreatureTypeWords("Octopus") {
		t.Fatal("precondition: Octopus is not in the creature-type vocabulary")
	}

	addMana(t, e, 0, "CC")
	opt, ok := findAbilityOption(e, octo, 0)
	if !ok {
		t.Fatalf("Acquisition Octopus's {2} reconfigure attach not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, bear)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(octo)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("octopus zone %s attached %d, want battlefield/bear %d", o.Zone, o.AttachedTo, bear)
	}
	types := e.Derived(octo).Types
	if e.IsCreature(octo) || slices.Contains(types, "Creature") {
		t.Fatalf("attached reconfigurer is still a creature: derived types %v", types)
	}
	if slices.Contains(types, "Octopus") {
		t.Fatalf("attached Acquisition Octopus keeps the Octopus creature subtype (CR 205.1a): derived types %v", types)
	}
	if !slices.Contains(types, "Artifact") || !slices.Contains(types, "Equipment") {
		t.Fatalf("attached Acquisition Octopus types %v, want Artifact and Equipment", types)
	}
	if effects.MatchesSpecFrom(e.G, "Octopus", octo, 0, 0) {
		t.Fatal("an Octopus subtype filter still matches the attached reconfigurer")
	}

	// Unattach: the Octopus is a creature again with its subtype back.
	addMana(t, e, 0, "CC")
	optU, ok := findAbilityOption(e, octo, 1)
	if !ok {
		t.Fatalf("unattach not offered while attached: %+v", e.Pending().Options)
	}
	submitChoices(t, e, optU.Index)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(octo).AttachedTo != 0 {
		t.Fatalf("after unattach octopus still attached to %d", e.G.Obj(octo).AttachedTo)
	}
	if types := e.Derived(octo).Types; !e.IsCreature(octo) || !slices.Contains(types, "Octopus") {
		t.Fatalf("unattached Acquisition Octopus types %v, want Creature and Octopus back", types)
	}
}
