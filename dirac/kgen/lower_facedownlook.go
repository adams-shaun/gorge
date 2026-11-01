package kgen

// faceDownLookOnly reports whether a Mode$ Continuous body is exactly the
// view-only look over face-down battlefield creatures. This is
// the shape of DSK Found Footage's printed static and MKM Lumbering Laundry's
// Effect static:
//
//	Mode$ Continuous | Affected$ Creature.faceDown+OppCtrl |
//	AffectedZone$ Battlefield | MayLookAt$ You
//
// gorge records the printed look as a ContinuousEffect with MayLookAt
// (engine/rules/layers.go:766-785), but every reader of that permission is
// either library-only (MayLookAtLibraryTop, layers_static.go:255-300), exile
// only (MayLookAtFaceDown, layers_static.go:302-340: a battlefield object
// returns false), the seat view (engine/view/card.go:353-374, :522-538;
// engine/view/view.go:66-72, :719) or the oracle runner's expectation read
// (engine/rules/oracle_run.go:213-255). No event, decision or legality reader
// changes the game state, so the exact kernel lowering is no static at all.
// The Effect route registers nothing either: mayLookAtExileGrantFromLine
// (engine/effects/staticline.go:202-232) accepts only AffectedZone$ Exile, so
// a battlefield body falls through effEffect to an unimplemented Note.
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
