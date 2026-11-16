package kgen

import "testing"

// livingEachForms: TargetsForEachPlayer$ True, TargetMax$ OneEach over
// SVar PlayerCountPlayers$Amount lowers to the different-controllers slot
// (lower_targeteachopponent.go livingEach); the opponents-only shapes keep
// their own lowering and a filter beyond battlefield permanents stays refused.
var livingEachForms = []formCase{
	{card: "Nils, Discipline Enforcer", form: "Creature, any controller", lir: "diff=true"},
	{card: "Battle at the Helvault", form: "Permanent.nonLand+nonSaga, any controller", lir: "diff=true"},
	{card: "Afterlife from the Loam", form: "a graveyard target stays refused", refused: "target filter is not opponent-controlled permanents"},
}

func TestLivingEachForms(t *testing.T) { checkForms(t, livingEachForms) }
