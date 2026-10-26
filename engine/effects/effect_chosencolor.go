package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// colourWordOf is the filter colour predicate word for a colour letter.
var colourWordOf = map[byte]string{'W': "White", 'U': "Blue", 'B': "Black", 'R': "Red", 'G': "Green"}

// frozenChosenColorSource locks an Effect-created replacement's ValidSource$
// to the colour chosen by THIS resolution (CR 611.2c: the affected set is
// fixed when the effect begins; CR 607.2 links the choice to that
// resolution). Forge's `Card.ChosenColor` reads the originating card's live
// choice, so a later ChooseColor on the same card (Prismatic Strands cast and
// then flashed back) would recolour every earlier effect. It returns the
// filter with `ChosenColor` replaced by the concrete colour word, and false
// when the filter does not read the choice or the source has none.
func frozenChosenColorSource(g *state.Game, source state.ObjID, validSource string) (string, bool) {
	if !strings.Contains(validSource, ".ChosenColor") {
		return "", false
	}
	s := g.Obj(source)
	if s == nil {
		return "", false
	}
	word, ok := colourWordOf[colourLetter(s.ChosenColor)]
	if !ok {
		return "", false
	}
	return strings.ReplaceAll(validSource, ".ChosenColor", "."+word), true
}
