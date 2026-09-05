package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// ColorsOf is an object's colours as WUBRG letters in that fixed order:
// the colours of its mana cost, or an explicit Colors: line for a card
// whose cost does not show them (a token, an artifact "that is green").
// Devoid makes a card colourless regardless (CR 702.114). A Face-less
// object (an ability, a copy of nothing) is colourless. Protection (rules)
// and the colour predicates read this rather than the face directly.
func ColorsOf(o *state.Object) string {
	if o == nil {
		return ""
	}
	f := o.Face()
	if f == nil || f.HasKeyword("Devoid") {
		return ""
	}
	set := map[byte]bool{}
	for _, r := range f.ManaCost {
		if strings.ContainsRune("WUBRG", r) {
			set[byte(r)] = true
		}
	}
	if len(set) == 0 && f.Colors != "" {
		for _, word := range strings.Split(strings.ToLower(f.Colors), ",") {
			switch strings.TrimSpace(word) {
			case "white":
				set['W'] = true
			case "blue":
				set['U'] = true
			case "black":
				set['B'] = true
			case "red":
				set['R'] = true
			case "green":
				set['G'] = true
			}
		}
	}
	var b strings.Builder
	for _, c := range "WUBRG" {
		if set[byte(c)] {
			b.WriteRune(c)
		}
	}
	return b.String()
}
