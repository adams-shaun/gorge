package kgen

import "testing"

// TestTapsForManaReflectedNonlandFilter: a reflected TapsForMana body over a
// NONLAND filter (Kinnan, Bonder Prodigy: "whenever you tap a nonland
// permanent for mana, add one mana of any type that permanent produced") goes
// to the same StTapReflect static the land filters do -- the kernel prices the
// extra on the tapped permanent's produced types exactly as gorge folds it per
// production. A land filter naming a clause the filter lowering does not hold
// (Extraplanar Lens's Land.sharesNameWith Imprinted) keeps refusing.
func TestTapsForManaReflectedNonlandFilter(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Kinnan, Bonder Prodigy", form: "reflected extra over Permanent.nonLand",
			lir: "TapExtra"},
		{card: "Extraplanar Lens", form: "reflected extra over Land.sharesNameWith Imprinted",
			refused: `TapsForMana reflected mana over ValidCard$ "Land.sharesNameWith Imprinted": not a land filter`},
	})
}
