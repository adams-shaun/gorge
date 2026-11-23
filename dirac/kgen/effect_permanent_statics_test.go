package kgen

import "testing"

// effectPermanentStaticsForms: a Duration$ Permanent Effect with no target and
// no RememberObjects$ lowers its several statics as one emblem.
var effectPermanentStaticsForms = []formCase{
	{card: "Saheeli, Filigree Master", form: "Effect Duration$ Permanent, two statics (pump + ReduceCost), one permanent emblem", lir: "emblem[0] trigger=0 statics=2"},
	{card: "A-Saheeli, Filigree Master", form: "Effect Duration$ Permanent, two statics (pump + ReduceCost), one permanent emblem", lir: "emblem[0] trigger=0 statics=2"},
}

func TestEffectPermanentMultiStaticEmblem(t *testing.T) { checkForms(t, effectPermanentStaticsForms) }
