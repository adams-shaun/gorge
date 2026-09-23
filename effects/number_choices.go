package effects

import (
	"strconv"

	"github.com/adams-shaun/gorge/decision"
)

// This file is the ONE home for the option list a ChooseNumber ask ranges
// over (task cli-20260923T060000Z-choose-number). Before it, the number list
// existed only inline in rules' as-enters ask (rules/cast.go etbOptions'
// "number" arm), so a mid-resolution ChooseNumber had no list at all and
// silently recorded 0. Here the list is named once; the asking primitive
// (effects/choose.go effChooseNumber) and the as-enters ask build their
// options from this function, so the two asks can never disagree.

// chooseNumberMaxOffer is the inclusive upper bound of the deterministic
// number list, matching the historical as-enters number ask (0..12). It is a
// build default, NOT the card's own range: Max$ (a dynamic or literal bound
// such as a mana pool or energy total) is owned by a separate ticket and
// remains unread here, so every ChooseNumber currently offers this fixed
// list. The bound is deliberately finite so the ask is answerable by a list
// pick; an unbounded card (Void's bare "Choose a number") stays a real ask
// over the values a game can plausibly need rather than a silent 0.
const chooseNumberMaxOffer = 12

// NumberChoices returns the deterministic number option list a ChooseNumber
// ask offers: 0..chooseNumberMaxOffer inclusive, in ascending order, each
// option carrying its value in Amount (the same "number" wire shape the
// as-enters ask already emitted) so the answer needs no label parsing.
func NumberChoices() []decision.Option {
	out := make([]decision.Option, 0, chooseNumberMaxOffer+1)
	for i := 0; i <= chooseNumberMaxOffer; i++ {
		out = append(out, decision.Option{Index: len(out), Kind: "number", Label: strconv.Itoa(i), Amount: i})
	}
	return out
}
