package botpolicy

import (
	stdmaps "maps"
	"slices"
)

// The typed value equalities boardsDiffer's verify path compares Board
// tables with, replacing reflect.DeepEqual at exactly its strictness: a nil
// and an empty non-nil slice or map DIFFER. TestBoardEqualMatchesDeepEqual
// holds them to DeepEqual and to the types' field counts.

// comparableEqual is DeepEqual for a pointer-free comparable value (Card,
// a life total): a field that stops being comparable fails to compile here.
func comparableEqual[V comparable](a, b *V) bool { return *a == *b }

func creatureEqual(a, b *Creature) bool {
	return a.Power == b.Power && a.Toughness == b.Toughness && a.Damage == b.Damage &&
		a.Tapped == b.Tapped && a.Controller == b.Controller &&
		a.Attacking == b.Attacking && a.Defender == b.Defender && a.Blocks == b.Blocks &&
		(a.Keywords == nil) == (b.Keywords == nil) && slices.Equal(a.Keywords, b.Keywords)
}

func commanderEqual(a, b *Commander) bool {
	return a.Casts == b.Casts && a.InCommandZone == b.InCommandZone &&
		(a.Damage == nil) == (b.Damage == nil) && stdmaps.Equal(a.Damage, b.Damage)
}

// AdaptersDiffer is the first difference between two Boards that two
// adapters built for the same seat at the same state (heuristic's
// BoardFromView and BoardFromGameInto), every table in entry order, or "".
// Order is a fact the policy reads: forgeHand takes the Cards order as the
// hand's, and the tap and target folds keep the first of equal entries, so
// two adapters that agree only as maps can still play differently. OwnDeck
// is not compared (the view half carries its own copy of the manifest;
// callers compare it by value); OwnLibrary is.
func AdaptersDiffer(a, b Board) string {
	if !a.OwnLibrary.Equal(b.OwnLibrary) {
		return "OwnLibrary"
	}
	a.OwnDeck = b.OwnDeck
	return boardsDiffer(a, b)
}
