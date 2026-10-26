package effects

// CR 205.2a: kindred is a card type. CR 308.3: "tribal" is its retired name.
// A Kindred Enchantment therefore contributes two distinct card types to
// Count$...$CardTypes, not one. Boggart Mischief is used as the inline
// carrier (the corpus prints the current Kindred spelling).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestEvalCountCardTypesCountsKindred(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	id := diffObject(t, g, 0, state.ZGraveyard,
		"Name:Boggart Mischief\nManaCost:1 B\nTypes:Kindred Enchantment\nOracle:x\n")

	// PRECONDITION: the card is in the counted graveyard and its face
	// really carries both type words -- otherwise the count below is
	// vacuous.
	o := g.Obj(id)
	if o == nil || o.Zone != state.ZGraveyard || o.Face() == nil {
		t.Fatalf("precondition: Boggart Mischief is not a graveyard object: %+v", o)
	}
	types := o.Face().Types
	if len(types) != 2 || types[0] != "Kindred" || types[1] != "Enchantment" {
		t.Fatalf("precondition: face types = %v, want [Kindred Enchantment]", types)
	}

	const body = "Count$ValidGraveyard Card.YouOwn$CardTypes"
	if got := EvalCount(h, c, body); got != 2 {
		t.Fatalf("%s = %d, want 2 (Kindred + Enchantment)", body, got)
	}
}
