package kgen

import "testing"

// filterHeadSameNameForms: Targeted.sameName+Other, every other permanent with
// the target's name (lower_filterreferents.go sameNameT0OtherFilter: one
// any-type PredSameNameT0 clause, PredN 1 excluding the target); the
// Triggered.sameName base lowers as FwSameName over the triggering object
// (T13b-2 remnants), and the off-battlefield Targeted.sameName spelling keeps
// refusing.
var filterHeadSameNameForms = []formCase{
	{card: "Echoing Decay", form: "PumpAll ValidCards$ Targeted.sameName+Other", lir: "PumpAll A=-2 B=-2 F={types=0x0 cmp(22 0 1  )}"},
	{card: "Homing Lightning", form: "DamageAll ValidCards$ Targeted.sameName+Other", lir: "DamageAll A=4 F={types=0x0 cmp(22 0 1  )}"},
	{card: "Izzet Staticaster", form: "activated DamageAll ValidCards$ Targeted.sameName+Other", lir: "DamageAll A=1 F={types=0x0 cmp(22 0 1  )}"},
	{card: "Legion's End", form: "Targeted.sameName over hand and graveyard", refused: "sharesNameWith Targeted sweep beyond the battlefield"},
	{card: "Verdant Succession", form: "Triggered.sameName library search", lir: "Search obj=TriggeredPlayer A=1 C=1 CS=Confirm F={types=0x0 cmp(204 0 0  ) prog[w8(0 0  )]}"},
}

func TestFilterHeadSameNameForms(t *testing.T) { checkForms(t, filterHeadSameNameForms) }
