package kgen

import "testing"

// TestGoddricLowering: Goddric, Cloaked Reveler's Celebration static (a Self
// override: RemoveCreatureTypes$ True without RemoveCardTypes$, AddType$
// Dragon, SetPower/SetToughness 4/4, Flying and the Dragon pump, gated on the
// celebration count) lowers to a creature-subtype strip that keeps the card
// types and adds the Dragon subtype.
func TestGoddricLowering(t *testing.T) {
	cs := lowerNamed(t, []string{"Goddric, Cloaked Reveler"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards", len(cs))
	}
	c := cs[0]
	if len(c.overrides) == 0 {
		t.Fatalf("no override lowered (refusals %v)", c.Refusals)
	}
	if len(c.Refusals) != 0 {
		t.Fatalf("refused: %v", c.Refusals)
	}
	o := c.overrides[0]
	if !o.RemoveCreatureSub {
		t.Errorf("RemoveCreatureSub not set")
	}
	if o.HasSetTypes || o.HasSetSubtypes {
		t.Errorf("the strip replaces the card types (HasSetTypes %v, HasSetSubtypes %v)", o.HasSetTypes, o.HasSetSubtypes)
	}
	if len(o.AddSubtypes) != 1 || o.AddSubtypes[0] != "Dragon" {
		t.Errorf("AddSubtypes %v, want [Dragon]", o.AddSubtypes)
	}
}
