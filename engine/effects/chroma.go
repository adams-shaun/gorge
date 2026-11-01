package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// chromaAllColours is Forge's ManaAtom.ALL_MANA_COLORS: every colour bit, the
// query a ChromaSource spelling with no <Colour> suffix reads ("that many
// colored mana symbols").
const chromaAllColours = cards.ColourWhite | cards.ColourBlue | cards.ColourBlack | cards.ColourRed | cards.ColourGreen

// chromaSourceMask parses the suffix a ChromaSource spelling carries after
// its head: "" (the bare form, every colour) or ".<Colour>", where <Colour>
// is a Forge colour word, a WUBRG letter or a two-letter hybrid spelling
// ("WU"), all through the shared colourLetter vocabulary. An unreadable
// suffix returns ok=false so the caller fails closed rather than counting a
// fake zero.
func chromaSourceMask(suffix string) (uint8, bool) {
	if suffix == "" {
		return chromaAllColours, true
	}
	tok, ok := strings.CutPrefix(suffix, ".")
	if !ok || tok == "" {
		return 0, false
	}
	return chromaColourMask(tok)
}

// chromaColourMask parses one Forge colour token to its colour bitmask: a
// colour word or letter, or two letters ORed together (Forge's
// ManaAtom.fromName reads a two-character name as its two characters'
// atoms, and an unknown character contributes nothing -- so "2B" is BLACK,
// exactly as Forge reads it). A token naming no colour at all fails closed;
// the corpus's ChromaSource spellings are all WUBRG words.
func chromaColourMask(tok string) (uint8, bool) {
	tok = strings.TrimSpace(tok)
	if len(tok) == 2 {
		mask := chromaColourBit(colourLetter(tok[:1])) | chromaColourBit(colourLetter(tok[1:]))
		if mask == 0 {
			return 0, false
		}
		return mask, true
	}
	letter := colourLetter(tok)
	if letter == 0 {
		return 0, false
	}
	return chromaColourBit(letter), true
}

// chromaColourBit maps a WUBRG letter to the cards.Colour* vocabulary's bit,
// the same mask ManaCostColours speaks.
func chromaColourBit(letter byte) uint8 {
	switch letter {
	case 'W':
		return cards.ColourWhite
	case 'U':
		return cards.ColourBlue
	case 'B':
		return cards.ColourBlack
	case 'R':
		return cards.ColourRed
	case 'G':
		return cards.ColourGreen
	}
	return 0
}

// chromaObjectCount counts one referenced object's mana symbols of the
// queried colours, through cards.ManaCostChroma's per-symbol Forge read. A
// face-down battlefield permanent has no mana cost (CR 708.5) and
// contributes nothing -- the same rule the battlefield devotion scan
// applies (devotionCount) -- and a missing or faceless object counts zero.
func chromaObjectCount(o *state.Object, mask uint8) int32 {
	if o == nil || o.FaceDownHidden() {
		return 0
	}
	f := o.Face()
	if f == nil {
		return 0
	}
	return cards.ManaCostChroma(f.ManaCost, mask)
}

// chromaSacrificedCount is chromaObjectCount's snapshot form: the
// Sacrificed$ChromaSource reader runs after the sacrifice's zone change, so
// it reads the SacrificedInfo capture (CR 608.2h) rather than a live object.
func chromaSacrificedCount(s state.SacrificedInfo, mask uint8) int32 {
	if s.FaceDown {
		return 0
	}
	return cards.ManaCostChroma(s.ManaCost, mask)
}
