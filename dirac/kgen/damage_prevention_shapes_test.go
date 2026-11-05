package kgen

import "testing"

// TestDamagePreventionShapesLower: Black Panther, Hope Enduring's printed
// "prevent all damage that would be dealt to it" R: line lowers as the
// CardDef.PreventDmgTo fact (lower_preventkw.go selfPreventRepl), and The
// Mindskinner's prevent-and-mill DamageDone replacement as PreventMillOpps
// (lower_replmult.go preventMillRepl).
func TestDamagePreventionShapesLower(t *testing.T) {
	checkCarrier(t, []string{"King T'Challa", "The Mindskinner"}, nil, []string{"PreventMillOpps"})
}
