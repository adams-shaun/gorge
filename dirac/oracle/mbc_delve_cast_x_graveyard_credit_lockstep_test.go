package oracle

import "testing"

// CR 601.2b with CR 702.66a: an announced X is bounded by what the payment
// can settle, and a spell with delve may exile a card from its controller's
// graveyard for each generic mana of the total cost, the announced X
// included. Delve too Deep ({X}{B}) on a small pool and a stocked graveyard:
// gorge's xAsk offers X = 0..5 on the pool plus the delve credit, while the
// kernel capped X at the pool alone and offered 0..1. The recording is pair
// 2 of the tail-08 focus pool dealt around Delve too Deep and Blood
// Divination (decision 355, options, gorge leading, on the full surface).
func TestMBCDelveCastXGraveyardCreditReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delve_cast_x_graveyard_credit")
}
