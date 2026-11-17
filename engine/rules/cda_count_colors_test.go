package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A characteristic-defining P/T count reads the board's layer-5 colours (CR
// 613.3: the colour layer is applied before the layer-7a CDA -- CR 613.4a --
// reads a colour word). An-Havva Constable's toughness is 1 plus the number
// of green creatures on the battlefield, and Hissing Quagmire's {1}{B}{G}
// activation makes it "a 2/2 black and green Elemental creature" for the
// turn: with no other green creature on the board the Constable reads 3, the
// animated Quagmire counting beside itself. The count once read the PRINTED
// colours (the board-wide layer-5 table is not readable mid-derivation), so
// the colourless-printed land did not count and the Constable read 2.
func TestCDACountReadsAnimatedManlandColours(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{
		lookup(t, reg, "An-Havva Constable"),
		lookup(t, reg, "Hissing Quagmire"),
	}, []*cards.Card{})
	land := moveByName(t, e, 0, "Hissing Quagmire", state.ZBattlefield)
	constable := moveByName(t, e, 0, "An-Havva Constable", state.ZBattlefield)

	addMana(t, e, 0, "CBG")
	submitChoices(t, e, animateAbilityOption(t, e, land).Index)
	settleActivation(t, e)

	// Precondition: the animation applied its colour set -- the land is black
	// and green now, not the colourless print.
	if got := e.Colors(land); got != "BG" {
		t.Fatalf("Hissing Quagmire colours after its animation = %q, want \"BG\"", got)
	}
	// Precondition: the only other permanent on the board is the Constable,
	// whose own green counts beside the Quagmire's.
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n != 2 {
		t.Fatalf("battlefield holds %d permanents, want 2 (the Constable and the animated Quagmire)", n)
	}
	if got := e.Toughness(constable); got != 3 {
		t.Fatalf("An-Havva Constable toughness beside its animated black-and-green manland = %d, want 3", got)
	}
}
