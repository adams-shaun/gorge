package pay

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// AbilityXColor is an activated ability's XColor$ ("Spend only black mana on
// X": Crypt Rats, Crimson Hellkite) as a Cost.XColor value: 1 + the pool
// index of the one colour its announced {X} must be paid in, 0 when the
// ability carries none. A multi-colour value ("Black,Red": "black and/or
// red") is no single colour, so it reads 0 and the {X} stays generic.
func AbilityXColor(sa *cards.SA) uint8 {
	v := sa.ParamStr(cards.PKXColor)
	if len(v) != 1 {
		// a colour word ("Black"); the corpus also spells one letter ("R")
		letters, ok := effects.ColorLetters(v)
		if !ok || len(letters) != 1 {
			return 0
		}
		v = letters[0]
	}
	switch v[0] {
	case 'W', 'U', 'B', 'R', 'G':
		return uint8(state.ManaIndex(v[0]) + 1)
	}
	return 0
}

// withAbilityXColor is c with sa's XColor$ folded in: the {X} the cost
// carries is paid in that colour once announced (Cost.WithX). c is returned
// unchanged when sa carries none or the cost has no {X}; otherwise the
// result is a fresh compiled cost (c is the shared read-only parse).
func withAbilityXColor(sa *cards.SA, c *CompiledCost) *CompiledCost {
	if c.X == 0 {
		return c
	}
	col := AbilityXColor(sa)
	if col == 0 {
		return c
	}
	k := c.Cost
	k.XColor = col
	return CompiledCostFromCost(k)
}
