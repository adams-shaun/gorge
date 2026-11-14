package effects

import "testing"

// TestParentTargetControlReferentsAreKnown: Searing Blaze's creature clause is
// recognised by the classifier, so the census reports nothing for it.
func TestParentTargetControlReferentsAreKnown(t *testing.T) {
	spec := "Creature.ControlledBy ParentTargetedController,Creature.ControlledBy ParentTarget"
	if got := UnknownPredicates(spec); len(got) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v; want none", spec, got)
	}
	if got := UnknownPredicates("Creature.ControlledBy NoSuchParent"); len(got) != 1 {
		t.Fatalf("precondition: an unrelated referent must stay unknown, got %v", got)
	}
}
