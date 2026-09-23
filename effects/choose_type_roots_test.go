package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestChooseTypeRootsOfLifeOffersOnlyAllowedBasicLandTypes drives the real
// ChooseLT SVar from the corpus. Roots of Life excludes three of the five
// basic land types, so the resolving ask must offer only Island and Swamp.
func TestChooseTypeRootsOfLifeOffersOnlyAllowedBasicLandTypes(t *testing.T) {
	card, ability := corpusSA(t, "Roots of Life", "ChooseLT")
	h := &chooseTypeHost{}
	h.g = state.NewGame(names(2))
	src := h.g.AddObject(card, 0).ID
	ctx := &Ctx{Source: src, Controller: 0, SVars: h.g.Obj(src).Face().SVars}
	Resolve(h, ctx, ability)
	if len(h.asks) != 1 {
		t.Fatalf("Roots of Life posed %d asks, want exactly one: %+v", len(h.asks), h.asks)
	}
	optionsAre(t, h.asks[0].Options, "Island", "Swamp")
}
