// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

func kwTransmute(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	// CR 702.53: transmute is a sorcery-speed hand activation. The
	// searched card has the source card's printed mana value.
	cost := strings.TrimSpace(param)
	sa, _ := parseSA("", "AB$ ChangeZone | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | SorcerySpeed$ True | Origin$ Library | Destination$ Hand | ChangeType$ Card.cmcEQ"+strconv.Itoa(int(f.Cmc()))+" | ChangeNum$ 1 | Keyword$ Transmute | SpellDescription$ Transmute "+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwTransmute, "Transmute") }
