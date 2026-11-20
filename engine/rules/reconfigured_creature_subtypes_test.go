package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestReconfiguredAuraHasNoCreatureSubtypes pins CR 205.1a on a real carrier:
// while Razorfield Ripper ("Artifact Creature -- Equipment Rhino") is attached
// by Reconfigure it is an artifact, not a creature (CR 702.151b), so the Rhino
// creature subtype correlated with the lost Creature card type goes with it.
// The derived type line is Artifact Equipment, and a Rhino subtype filter no
// longer reads the attached permanent as one.
func TestReconfiguredAuraHasNoCreatureSubtypes(t *testing.T) {
	e, ripper, bear, _ := ripperBoard(t)

	// Precondition: the printed card is a Creature with the Rhino subtype, so
	// the assertions below are about the Reconfigure switch and not an empty
	// list.
	printed := e.G.Obj(ripper).Face().Types
	if !slices.Contains(printed, "Creature") || !slices.Contains(printed, "Rhino") {
		t.Fatalf("printed Razorfield Ripper types %v, want Creature and Rhino", printed)
	}
	if e.G.Obj(ripper).AttachedTo != 0 {
		t.Fatalf("ripper starts attached to %d, want unattached", e.G.Obj(ripper).AttachedTo)
	}

	addMana(t, e, 0, "CC")
	opt, ok := findAbilityOption(e, ripper, 0)
	if !ok {
		t.Fatal("Razorfield Ripper's {2} reconfigure attach not offered")
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, bear)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(ripper)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("ripper zone %s attached %d, want battlefield/bear %d", o.Zone, o.AttachedTo, bear)
	}
	types := e.Derived(ripper).Types
	if e.IsCreature(ripper) || slices.Contains(types, "Creature") {
		t.Fatalf("attached reconfigurer is still a creature: derived types %v", types)
	}
	if !slices.Contains(types, "Artifact") || !slices.Contains(types, "Equipment") {
		t.Fatalf("attached reconfigurer types %v, want Artifact and Equipment", types)
	}
	if slices.Contains(types, "Rhino") {
		t.Fatalf("attached reconfigurer keeps the Rhino creature subtype (CR 205.1a): derived types %v", types)
	}
	if effects.MatchesSpecFrom(e.G, "Rhino", ripper, 0, 0) {
		t.Fatal("a Rhino subtype filter still matches the attached reconfigurer")
	}
}
