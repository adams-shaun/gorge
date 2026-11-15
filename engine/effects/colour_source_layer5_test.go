package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestColourSourceReadsLayer5Colours: the <Colour>Source words (Jaya's and
// Torbran's Card.RedSource) read the object's current colours, layer 5
// included, as the bare colour words do (CR 105.2: a red source is a source
// that is red now). They once read the printed face, so a recoloured white
// creature failed Card.RedSource while it matched Card.Red.
func TestColourSourceReadsLayer5Colours(t *testing.T) {
	g := state.NewGame(names(2))
	o := g.AddObject(mkCard(t, "Name:Pale Knight\nManaCost:1 W\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	match := func(spec string, sc SpecContext) bool { return MatchesSpecCtxPtr(g, spec, o.ID, &sc) }

	printed := SpecContext{}
	// Precondition: printed white, so the layer-5 cases below differ.
	if !match("Card.WhiteSource", printed) || match("Card.RedSource", printed) {
		t.Fatal("precondition: the printed white creature is a white source and not a red one")
	}
	red := SpecContext{}
	red.Layers.DerivedColors = []ObjectColors{{ID: o.ID, Mask: ColorMaskFromLetters("R")}}
	if !match("Card.Red", red) {
		t.Fatal("precondition: the bound layer-5 table makes the creature red for Card.Red")
	}
	if !match("Card.RedSource", red) {
		t.Error("Card.RedSource misses a creature layer 5 made red")
	}
	if match("Card.WhiteSource", red) {
		t.Error("Card.WhiteSource matches a creature layer 5 made red only")
	}
	none := SpecContext{}
	none.Layers.DerivedColors = []ObjectColors{{ID: o.ID, Mask: 0}}
	if !match("Card.ColorlessSource", none) {
		t.Error("Card.ColorlessSource misses a creature layer 5 made colourless")
	}
	if match("Card.ColorlessSource", printed) {
		t.Error("Card.ColorlessSource matches the printed white creature")
	}
}
