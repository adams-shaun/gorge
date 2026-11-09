package kgen

import (
	"strings"
	"testing"
)

// The three mana shapes of the artifact-rider family stay refused by name, each
// for the reason the measurement found. A lockstep sweep with the shape lowered
// diverged (or the kernel has no mechanism), so a card here is lowered only
// when the mechanism and a lockstep repro land together:
//   - a filter's cost that sacrifices its source (Omni-Cheese Pizza): the
//     prefix search would sacrifice on its State copies;
var manaRiderForms = []formCase{
	{card: "Boommobile", form: "an artifact's triggered mana restricted to activated abilities", lir: "ManaChoice A=4 C=1 CS=WUBRG F={types=0x0 special=0x20}"},
}

func TestManaRiderShapes(t *testing.T) {
	checkForms(t, manaRiderForms)
	// Omni-Cheese Pizza is refused when the card is built (filterRefusal)
	for _, c := range lowerPoolBuilt(t, []string{"Omni-Cheese Pizza"}) {
		if c.name != "Omni-Cheese Pizza" {
			continue
		}
		if len(c.refusals) != 1 || !strings.Contains(c.refusals[0].Reason, "filter mana ability with a sacrifice") {
			t.Errorf("Omni-Cheese Pizza: want the sacrifice-filter refusal, got %v", c.refusals)
		}
		return
	}
	t.Error("Omni-Cheese Pizza: not built")
}
