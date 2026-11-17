package kgen

import "testing"

// filterHeadSameNameForms: Targeted.sameName+Other, every other permanent with
// the target's name (lower_filterreferents.go sameNameT0OtherFilter: one
// any-type PredSameNameT0 clause, PredN 1 excluding the target); the
// Triggered.sameName and off-battlefield Targeted.sameName spellings keep
// refusing.
var filterHeadSameNameForms = []formCase{
	{card: "Echoing Decay", form: "PumpAll ValidCards$ Targeted.sameName+Other", lir: "PumpAll A=-2 B=-2 F={types=0x0 cmp(22 0 1  )}"},
	{card: "Homing Lightning", form: "DamageAll ValidCards$ Targeted.sameName+Other", lir: "DamageAll A=4 F={types=0x0 cmp(22 0 1  )}"},
	{card: "Izzet Staticaster", form: "activated DamageAll ValidCards$ Targeted.sameName+Other", lir: "DamageAll A=1 F={types=0x0 cmp(22 0 1  )}"},
	{card: "Legion's End", form: "Targeted.sameName over hand and graveyard", refused: "sharesNameWith Targeted sweep beyond the battlefield"},
	{card: "Verdant Succession", form: "Triggered.sameName library search", refused: `unknown filter head "Triggered"`},
}

func TestFilterHeadSameNameForms(t *testing.T) { checkForms(t, filterHeadSameNameForms) }
