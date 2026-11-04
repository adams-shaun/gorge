package kgen

import "testing"

// Gowron, Eager for Battle's gate: Sacrificed$Valid Card.Klingon reads the
// sacrifice-time snapshot's derived creature types (gorge sacrificedValidCount,
// effects/count_ref.go). The LIR must keep the subtype on the RefCount arm.
func TestGowronSacrificedTypeLowering(t *testing.T) {
	checkForms(t, []formCase{{
		card: "Gowron, Eager for Battle",
		form: "Sacrificed$Valid Card.Klingon",
		lir:  "RefCount({types=0x0 sub=Klingon},Sacrificed)",
	}})
}

// TestSacrificedTypeSpecShape: exactly Card.<creature type> and
// Creature.<creature type> lower; everything else keeps the refusal.
func TestSacrificedTypeSpecShape(t *testing.T) {
	lc := &lowerCtx{p: &pool{}, known: func(string) bool { return true }}
	for _, spec := range []string{"Card.Klingon", "Creature.Klingon"} {
		if _, ok := lc.sacrificedTypeFilter(spec); !ok {
			t.Errorf("%q: not lowered", spec)
		}
	}
	for _, spec := range []string{
		"Card.Artifact",            // a card type, not a creature type
		"Creature",                 // no subtype
		"Permanent.Klingon",        // the Permanent base
		"Card.Klingon+Other",       // another property
		"Card.IsSuspected",         // the designation, lowered by its own arm
		"Creature.Klingon+YouCtrl", // a controller clause
	} {
		if _, ok := lc.sacrificedTypeFilter(spec); ok {
			t.Errorf("%q: lowered, want refused", spec)
		}
	}
}
