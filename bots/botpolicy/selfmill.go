package botpolicy

import "github.com/adams-shaun/gorge/engine/effects"

// The self-mill guard. A name-keyed self-mill (Card.NameKeyedSelfMill:
// Demonic Consultation, Spoils of the Vault, Tainted Pact) digs its caster's
// own library away until a card matches a name. The bot cannot aim that
// name inside its library, so the dig's honest worst case is the whole
// library, and a seat that casts it with no way to win on an empty library
// decks itself at its next draw: the cEDH goldfish runs lost 14% of the
// bot's games to its own cards, Consultation the commonest.
//
// So the cast is refused unless it wins: the seat controls a permanent that
// wins at a draw from an empty library (EmptyLibraryWinDraw: Laboratory
// Maniac), or it holds another castable card that wins on an empty library
// (Thassa's Oracle, or Laboratory Maniac to cast) and its mana pays both
// casts now. A "safe library" exemption has no honest reading for this
// class: the stop is a name the bot does not choose toward its library, so
// no library size bounds the loss. Every fact is read off the cards' IR
// (engine/effects castselfmill.go), never a card name or Forge's AI hints.

// selfMillVeto reports whether the guard refuses casting c.
func (b Board) selfMillVeto(c Card) bool {
	return c.NameKeyedSelfMill && !b.emptyLibraryWinReady(c)
}

// castVetoed is every hard cast refusal that ignores the decision's shape:
// the opt-in AIHints RemoveDeck veto and the self-mill guard. The cast
// ranking, the subset search's candidate table and the tap gate all ask it,
// so none of them prices or taps for a card the others refuse.
func (b Board) castVetoed(c Card) bool {
	return b.aiHintsVetoCast(c) || b.selfMillVeto(c)
}

// emptyLibraryWinReady reports whether the seat can win once spell, a
// name-keyed self-mill, has emptied its library: it controls a draw-
// replacement winner, or another castable empty-library winner whose cost,
// added to spell's, its mana reaches now.
func (b Board) emptyLibraryWinReady(spell Card) bool {
	cs := b.Cards.Values()
	for i := range cs {
		w := &cs[i]
		if w.EmptyLibraryWin == 0 {
			continue
		}
		if w.OnBattlefield {
			if w.EmptyLibraryWin == effects.EmptyLibraryWinDraw {
				return true
			}
			continue
		}
		if !w.Castable || w.NameKeyedSelfMill {
			continue
		}
		pips := colourPips(spell.ManaCost)
		wp := colourPips(w.ManaCost)
		var coloured int32
		for k := range pips {
			pips[k] += wp[k]
			coloured += pips[k]
		}
		generic := CmcOf(spell.ManaCost) + CmcOf(w.ManaCost) - coloured
		if b.manaReaches(pips, max(generic, 0)) {
			return true
		}
	}
	return false
}
