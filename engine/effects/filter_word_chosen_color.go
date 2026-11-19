package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

func chosenColorMatches(g *state.Game, o *state.Object, sc SpecContext) bool {
	if sc.Source == 0 {
		return false
	}
	_, col := sourceChosen(g, sc)
	chosen := colourLetter(col)
	return chosen != 0 && strings.Contains(colorsCtx(o, &sc), string(chosen))
}

// chosenColorMatchesKind routes the two chosen-colour words (ChosenColor and
// AnyChosenColor) to their readers.
func chosenColorMatchesKind(kind wordKind, g *state.Game, o *state.Object, sc SpecContext) bool {
	if kind == wordAnyChosenColor {
		return anyChosenColorMatches(g, o, sc)
	}
	return chosenColorMatches(g, o, sc)
}

// anyChosenColorMatches is Forge's CardStateProperty AnyChosenColor
// (Tablet of the Guilds' `ValidCard$ Card.AnyChosenColor` trigger): the
// candidate shares AT LEAST ONE colour with the source's chosen colours --
// the ChosenColor/ChosenColors union (Forge keeps both answers on one
// chosenColors list, its getChosenColor() being the first element). The
// candidate's colours are read through colorsCtx, the same layer-bound
// read chosenColorMatches takes, and a source with no recorded choice
// fails closed (contextPredicateBound guards the unbound case, so the '!'
// spelling cannot match every object either).
func anyChosenColorMatches(g *state.Game, o *state.Object, sc SpecContext) bool {
	if sc.Source == 0 {
		return false
	}
	mask := chosenColoursMask(g, sc.Source)
	if mask == 0 {
		return false
	}
	return ColorMaskFromLetters(colorsCtx(o, &sc))&mask != 0
}
