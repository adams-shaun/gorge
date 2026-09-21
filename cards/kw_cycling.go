// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwCycling(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	cost := strings.TrimSpace(param)
	sa, _ := parseSA("", "AB$ Draw | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | NumCards$ 1 | Keyword$ Cycling | SpellDescription$ Cycling "+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwCycling, "Cycling") }
