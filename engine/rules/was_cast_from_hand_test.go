package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Forge's generic wasCastFromHand (CardProperty wasCastFrom<Zone>, no
// Your/Their owner scope), the spelling Wash Away's Card.!wasCastFromHand now
// carries, reads the same hand cast as wasCastFromTheirHand: a hand spell
// passes the negated word, a graveyard (flashback) cast fails it.
func TestWasCastFromHandReadsHandOrigin(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	fb1 := graveCard(e, card(t, cbcFlashbackSrc), 1, 1)
	hand1 := handCard(e, card(t, cbcSpellSrc), 1)
	if e.G.Obj(fb1).Zone != state.ZGraveyard || e.G.Obj(hand1).Zone != state.ZHand {
		t.Fatal("precondition: fixture zones graveyard/hand")
	}
	cbcEffect(t, e, "Mode$ CantBeCast | ValidCard$ Card.!wasCastFromHand | Caster$ Opponent", "", nil)
	if n := cbcNotes(e); n != 0 {
		t.Fatalf("unimplemented Note emitted %d times", n)
	}
	if !e.castRestricted(1, fb1) {
		t.Fatal("opponent's graveyard spell not restricted")
	}
	if e.castRestricted(1, hand1) {
		t.Fatal("opponent's hand spell restricted")
	}
}
