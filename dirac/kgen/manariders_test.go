package kgen

import (
	"strings"
	"testing"
)

// The three mana shapes of the artifact-rider family stay refused by name, each
// for the reason the measurement found. A lockstep sweep with the shape lowered
// diverged (or the kernel has no mechanism), so a card here is lowered only
// when the mechanism and a lockstep repro land together:
//   - an artifact's triggered mana restricted to activating abilities (Boommobile):
//     the resolving restricted batch has no activated half and the artifact tag
//     is not recorded on a resolving AddMana;
//   - a filter's cost that sacrifices its source (Omni-Cheese Pizza): the
//     prefix search would sacrifice on its State copies;
//   - a {T} ability adding several colours under a spell restriction (Troyan,
//     Gutsy Explorer): with the shape lowered, a filter land's activation search
//     beside it offers fewer plays than gorge's.
var manaRiderForms = []formCase{
	{card: "Boommobile", form: "an artifact's triggered mana restricted to activated abilities", refused: "typed mana"},
	{card: "Troyan, Gutsy Explorer", form: "two colours at once under a spell restriction", refused: "several colours at once under a spend restriction"},
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
