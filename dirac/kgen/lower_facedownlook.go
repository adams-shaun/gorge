package kgen

// faceDownLookOnly reports whether a Mode$ Continuous body is exactly the
// view-only look over face-down battlefield creatures. This is
// the shape of DSK Found Footage's printed static and MKM Lumbering Laundry's
// Effect static:
//
//	Mode$ Continuous | Affected$ Creature.faceDown+OppCtrl |
//	AffectedZone$ Battlefield | MayLookAt$ You
//
// gorge records the look as a ContinuousEffect with MayLookAt
// (engine/rules/layers.go's MayLookAt branch); MayLookAtFaceDown
// (layers_static.go) answers for a face-down battlefield permanent when the
// grant names Battlefield, and the seat view (engine/view/card.go) shows that
// seat the face. The Effect route registers the same grant with the Effect's
// lifetime (effects.mayLookAtGrantFromLine). MayLookAtLibraryTop, the seat
// view and the oracle runner's expectation read are the other consumers. No
// event, decision or legality reader changes the game state, so the exact
// kernel lowering is no static at all.
//
// The kernel's Projection is raw state and its seat view (dirac/feat) does
// not model per-seat look permissions -- the same choice the library-top
// MayLookAt lowering makes (lower_continuous.go). The look is information
// only, so TestMaylookatLockstep plays the paired games decision for decision
// and the activation lowers to an empty program.
//
// The whitelist is exact: a body carrying any other rules key, another
// Affected$ spelling, another zone or another MayLookAt$ controller keeps the
// existing named refusal.
func faceDownLookOnly(p map[string]string) bool {
	for k := range p {
		switch k {
		case "Mode", "Affected", "AffectedZone", "MayLookAt", "Description", "Secondary":
		default:
			return false
		}
	}
	if p["AffectedZone"] != "Battlefield" {
		return false
	}
	switch p["MayLookAt"] {
	case "You", "True":
	default:
		return false
	}
	switch p["Affected"] {
	case "Creature.faceDown+OppCtrl", "Creature.faceDown+YouDontCtrl":
		return true
	}
	return false
}
