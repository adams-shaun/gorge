package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the AdventureCard filter word (CR 715.1; gorge
// wordAdventureCard). It reads the object's own CARD, never the current face:
// true iff the card's layout is Adventure (face 1 an instant or sorcery
// Adventure, which is all the table's FaceAdventure cards). Both faces of an
// adventurer carry FaceMode FaceAdventure, so the Adventure half on the stack
// and the creature or land half match alike. A layer-1 copy reads its own
// card (gorge reads o.Card, not the copied face).

// DebugAdventureCardNever is the mutation hook: adventureCard answers false.
var DebugAdventureCardNever bool

func adventureCard(k *prep, c CardID) bool {
	if DebugAdventureCardNever {
		return false
	}
	return k.t.Cards[c].FaceMode == kop.FaceAdventure
}
