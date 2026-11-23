// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwLivingMetal expands K:Living metal (CR 702.161a): "During your turn, this
// permanent is an artifact creature in addition to its other types." The
// expansion is ONE layer-4 type-adding continuous static on the permanent
// itself whose Condition$ PlayerTurn gate is the "during your turn" clause:
// engine/rules/layers_static.go's continuousConditionHolds holds it while the
// static's controller is the active player, and the layer walk re-derives the
// type list each turn change, so the permanent stops being a creature when the
// opponent's turn begins. AddType adds to, never replaces, the Vehicle's own
// types, so a Vehicle that is already a creature or artifact is unchanged.
func kwLivingMetal(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("S", k) {
		return
	}
	p := parseParams("Mode$ Continuous | Affected$ Card.Self | AddType$ Artifact & Creature | Condition$ PlayerTurn" +
		" | Description$ Living metal (During your turn, this permanent is an artifact creature in addition to its other types.)")
	p["KeywordLine"] = k
	f.Statics = append(f.Statics, Static{Mode: "Continuous", Params: p})
}

func init() { registerKeyword(kwLivingMetal, "Living metal") }
