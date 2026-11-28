package kgen

import "testing"

// samenameOtherTypeForms: a Targeted filter whose base names another type
// before Other+sameName (Echoing Courage's "target creature and all other
// creatures with the same name as that creature") names the target as the
// referent and excludes it, the type kept: one PredSameNameT0 clause with
// PredN 1, as the bare Targeted.sameName+Other form lowers
// (lower_filterreferents.go sameNameT0OtherFilter). Before the general
// spelling was parsed, Other stayed the source-relative flag and the targeted
// creature matched its own name again (Fate Unraveler got +2/+2 twice).
var samenameOtherTypeForms = []formCase{
	{card: "Echoing Courage", form: "PumpAll ValidCards$ Targeted.Creature+Other+sameName", lir: "PumpAll A=2 B=2 F={types=0x2 cmp(22 0 1  )}"},
}

func TestSameNameOtherTypeForms(t *testing.T) { checkForms(t, samenameOtherTypeForms) }
