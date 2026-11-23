package oracle

import "testing"

// Skullmane Baku ({3}{B}{B}) with Akoum Refuge, Holdout Settlement (its any-colour
// mana taps an untapped creature), School of the Unseen and two Islands: the two
// engines disagreed on whether the cast is offered (tail-42-c000 pair 60, decision 400).
func TestMBCSkullmaneBakuReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "skullmane_baku_tapx_creature_mana_cast")
}
