// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwEncore(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	// param is "<cost>" ("3 R"), occasionally followed by further
	// colon-separated fields no corpus line carries; the first field is
	// the cost. The expansion mirrors the C21 oracle shape ("Encore
	// <cost> (<cost>, Exile this card from your graveyard: For each
	// opponent, create a token copy that attacks that opponent this turn
	// if able. They gain haste. Sacrifice them at the beginning of the
	// next end step. Activate only as a sorcery.)"): one AB$ ability in
	// the graveyard whose cost is the printed cost plus exiling the card
	// itself (ExileFromGrave<1/CARDNAME>), resolved by effects.Encore
	// (encore.go): one CardToken copy per opponent, haste granted, and
	// one end-step delayed sacrifice per copy. The tokens' "attacks that
	// opponent this turn if able" is NOT enforced -- this build has no
	// attack-requirement machinery for it (recorded in the ticket
	// report's Issues), the copy is otherwise exact.
	cost, _, _ := strings.Cut(param, ":")
	sa, _ := parseSA("", "AB$ Encore | Cost$ "+cost+" ExileFromGrave<1/CARDNAME> | ActivationZone$ Graveyard | SorcerySpeed$ True | Keyword$ Encore | SpellDescription$ Encore "+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwEncore, "Encore") }
