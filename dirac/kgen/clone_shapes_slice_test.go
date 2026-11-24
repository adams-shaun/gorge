package kgen

import "testing"

// cloneShapesSliceCarriers are the cards of the gap-clone-shapes umbrella whose
// only blocker was CopyPermanent SetCreatureTypes$ ("except it's a Frog / a
// 4/4 black Zombie", CR 707.9b): the exception strips the copy's creature
// types, keeps its other subtypes, and adds the named types
// (kop.Override.RemoveCreatureSub read as a copy exception).
var cloneShapesSliceCarriers = []string{
	"Ardyn, the Usurper", "Croaking Counterpart", "God-Pharaoh's Gift", "Hashaton, Scarab's Fist",
	"Hour of Eternity", "Preston, the Vanisher", "The Scarab God",
}

func TestCloneShapesSlice(t *testing.T) {
	for _, name := range cloneShapesSliceCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// SetCreatureTypes$ lowers to a RemoveCreatureSub copy exception that adds the
// named creature type; a word that is no creature type, or a changeling copy
// beside it, stays refused.
func TestCloneShapesSetCreatureTypesException(t *testing.T) {
	c := lowerNamedCard(t, "Croaking Counterpart")
	var found bool
	for _, o := range c.overrides {
		if o.RemoveCreatureSub && len(o.AddSubtypes) == 1 && o.AddSubtypes[0] == "Frog" && o.HasSetPT && o.SetP == 1 && o.SetT == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("Croaking Counterpart overrides = %+v, want a RemoveCreatureSub exception adding Frog", c.overrides)
	}
}
