package botpolicy

import "github.com/adams-shaun/gorge/engine/cards"

// This file holds two small cast
// rules over Forge's AI-hint vocabulary, re-expressed in gorge's own words.
// Nothing here is translated from Forge Java.
//
// The rules are off by default. They are armed only through AIHints, which an
// opt-in bot constructor supplies; every zero-value configuration leaves the
// cast scorer in cast.go byte-identical.

// AIHints selects the A3 Forge AI-hint rules. The zero value is off.
type AIHints struct {
	// Enabled arms both rules: a PlayMain1 card earns the precombat timing
	// bonus in the first main phase, and a RemoveDeck card is never chosen.
	Enabled bool
}

// On reports whether the A3 rules are armed.
func (h AIHints) On() bool { return h.Enabled }

// aiHintsPlayMain1Bonus is the score a PlayMain1 card earns in the first
// main phase when its condition holds. It is deliberately on the same scale
// as a creature body (a 2/2 is worth 38), so the hint can separate a card
// from a comparable alternative; it is not a hard rule, and a C9 threshold
// can still hold the card when the profile asks for one.
const aiHintsPlayMain1Bonus int32 = 40

// aiHintsVetoCast reports whether the armed A3 rules refuse this card
// outright. Forge's RemoveDeck hint means "the AI does not know how to play
// this card", so the opt-in policy never chooses it as a cast.
func (b Board) aiHintsVetoCast(c Card) bool {
	return b.AIHints.On() && c.RemoveDeck
}

// aiHintsCastBonus is the timing bonus the A3 PlayMain1 rule adds in the
// caster's own first main phase. firstMain must already be the active
// player's first main (callers pass b.FirstMain && b.MyTurn): the hint is
// "play this precombat", which is the active player's window, not an
// opponent's first main. A value outside the closed set earns nothing; the
// own-creature condition is Forge's default for TRUE and every unknown
// spelling, and OPPONENTCREATURES names the other side explicitly.
func (b Board) aiHintsCastBonus(c Card, firstMain bool, ownCreatures, oppCreatures int32) int32 {
	if !b.AIHints.On() || !firstMain || c.PlayMain1 == "" {
		return 0
	}
	switch c.PlayMain1 {
	case cards.PlayMain1Always:
		return aiHintsPlayMain1Bonus
	case cards.PlayMain1OpponentCreatures:
		if oppCreatures > 0 {
			return aiHintsPlayMain1Bonus
		}
	default:
		if ownCreatures > 0 {
			return aiHintsPlayMain1Bonus
		}
	}
	return 0
}
