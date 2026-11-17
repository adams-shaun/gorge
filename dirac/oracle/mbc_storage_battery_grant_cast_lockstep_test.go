package oracle

import "testing"

// CR 601.2g-h, CR 605.3a: p1 holds Kami of Fire's Roar ({3}{R}) and Pyretic
// Charge (plot {3}{R}) beside a Plaza of Heroes, a Mercadian Bazaar holding two
// storage counters and a creature wearing Paradise Mantle: the Plaza's {C}, the
// Mantle's one mana of any colour and the Bazaar's two {R} pay each (decision 485
// of tail-26 focus-gs pair 4).
func TestMBCKamiOfFiresRoarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_beside_grant_cast_withheld")
}
