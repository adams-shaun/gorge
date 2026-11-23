package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFranticFireboltGraveyardTallyReadsGraveyardFace pins Frantic Firebolt's
// X ("instant cards, sorcery cards, and/or have an Adventure" in your
// graveyard) to the characteristics a card has IN the graveyard, never the
// face it was cast as: CR 400.7 (a zone change makes a new object), CR 715.4
// (off the stack an Adventure card has only its normal characteristics) and
// CR 712.8a (a back-face spell that leaves the stack without becoming a
// permanent is its front face again). Each card is cast on its alternate
// face (FaceIdx 1 on the stack) and then moved to the graveyard through the
// event fold, the way a countered or fizzled spell gets there.
func TestFranticFireboltGraveyardTallyReadsGraveyardFace(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const body = "Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn,Card.AdventureCard+YouOwn"
	for _, tc := range []struct {
		name string
		card string
		want int32
	}{
		// The Adventure half is an instant; the card in the graveyard is a
		// creature that "has an Adventure": counted once, not zero, not twice.
		{"countered Adventure spell", "Bonecrusher Giant", 1},
		// A land-back MDFC's front is a sorcery.
		{"sorcery-front MDFC cast as its back", "Agadeem's Awakening", 1},
		// A creature-front MDFC is a creature in the graveyard: not counted.
		{"creature-front MDFC cast as its back", "Akoum Warrior", 0},
		{"plain creature", "Grizzly Bears", 0},
		{"plain instant", "Shock", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := state.NewGame([]string{"you", "them"})
			c, ok := reg.Lookup(tc.card)
			if !ok {
				t.Fatalf("corpus has no %q", tc.card)
			}
			o := g.AddObject(c, 0)
			id := o.ID
			g.SetZone(state.ZHand, 0, []state.ObjID{id})
			g.Obj(id).Zone = state.ZHand
			events.Move(g, id, state.ZHand, state.ZStack)
			if len(c.Faces) == 2 {
				g.Obj(id).SetFaceIdx(1) // cast as the Adventure / back face
			}
			events.Move(g, id, state.ZStack, state.ZGraveyard)
			o = g.Obj(id)
			if o.Zone != state.ZGraveyard || len(g.Zone(state.ZGraveyard, 0)) != 1 {
				t.Fatalf("precondition: %q is in zone %v, graveyard holds %d", tc.card, o.Zone, len(g.Zone(state.ZGraveyard, 0)))
			}
			if got := EvalCount(&fakeHost{g: g}, &Ctx{Controller: 0}, body); got != tc.want {
				t.Fatalf("%s: tally = %d, want %d (graveyard face %q, FaceIdx %d)", tc.card, got, tc.want, o.Face().Name, o.FaceIdx)
			}
		})
	}
}
