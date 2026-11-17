package oracle

import "testing"

// CR 603.6a, 303.4f: Brago, King Eternal exiles up to four of its controller's
// nonland permanents (here Vow of Flight, an Aura, then Brago itself) and
// returns "those cards" together. The return walks the cards the trigger
// remembered, in the order they were exiled, so the Aura's host election is
// asked first, while Brago is still in exile. gorge walked Brago first,
// because the trigger's own capture (Brago dealt the damage) led the
// remembered list and, Brago being in the source's written list too, was kept.
// The recording is the first divergence of tail-23 pair 37 on the full
// surface: decision 308, gorge leading, a projection of Brago's zone.
func TestMBCBragoReturnOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "brago_return_order_aura_first")
}
