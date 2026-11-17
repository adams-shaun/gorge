package oracle

import (
	"testing"
)

// A window answer of a colour only a granted ability makes activates that
// grant (gorge's choose-a-mana-ability ask, printed first, the grant when
// only it makes the owed colour), so the tap floats the granted ability's
// production: Gift of Paradise's "{T}: Add two mana of any one color" on a
// Forest is two mana of the answer, not one. The kernel took the colour from
// the grant and the size from the land's printed ability, leaving a hybrid
// pip one short and the payment without a plan (decision 626, surface
// autopay). The recording is on a custom pool, the table its own decks need:
// the tail buckets a synthetic pool deals re-cut whenever the lowered card
// set moves, and the four cards this game needs stopped sharing one.
func TestMBCGrantedTwoManaWindowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_two_mana_hybrid_window_yields_one")
}
