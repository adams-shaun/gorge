package kop

// ExtLibPlace is the owner's CR 401.4 arrangement of cards one effect puts at
// a library position together ("If an effect puts two or more cards in a
// specific position in a library at the same time, the owner of those cards
// may arrange them in any order"; gorge effects.libraryPlacementOrder, the
// KArrange of Kind "library_placement" a ChangeZoneAll sweep or a multi-card
// ChangeZone poses after its moves). It brackets the move it follows:
//
//	C = PlaceArmTop / PlaceArmBottom (before the move): clears the
//	    per-resolution tally and starts counting the cards the next moves put
//	    into each owner's library, owners in first-placed order;
//	C = PlaceAsk (after the move): for each owner with two or more cards in
//	    the tally, in that order, the OWNER orders them (ChArrange picks over the
//	    cards at the top, or at the bottom, of that library; the first pick
//	    nearest the top; Min == Max == the count, no Done), then the tally ends.
//
// Nothing is asked when no owner received two cards. dirac/act_libplace.go.
const ExtLibPlace ExtOp = 301

// ExtLibPlace's C: the arm and the ask.
const (
	PlaceArmTop    = 0 // count the library placements that follow; they go to the top
	PlaceArmBottom = 1 // the same, to the bottom
	PlaceAsk       = 2 // order each owner's counted cards
)

func init() { extOpNames[ExtLibPlace] = "LibPlace" }
