package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/effects/params"
)

// ManaSymbols is the set of single-letter mana symbols the engine recognises
// in a Produced$ value (params.ManaSymbols).
const ManaSymbols = params.ManaSymbols

// ComboColours parses a Produced$ "Combo <colour> ..." value into the colours
// it names (params.ComboColours, the one classifier rules' colour ask and
// effMana's fail-closed walk share).
func ComboColours(produced string) (cols []string, ok bool) { return params.ComboColours(produced) }

// IsComboAllocation reports whether produced is a "Combo ..." production whose
// Amount$ units are allocated one at a time (a card's own "add two mana in any
// combination of colors") rather than all of one chosen colour. Callers apply
// their own AnyDifferent and Chosen gates: those shapes constrain the
// allocation (distinct colours, or an as-enters choice) beyond this prefix.
func IsComboAllocation(produced string) bool {
	return strings.HasPrefix(strings.TrimSpace(produced), "Combo ")
}
