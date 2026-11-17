package oracle

import "testing"

// CR 106.6: Somberwald Sage's three mana "spend this mana only to cast creature
// spells" pay Fogwalker's {1}{U}; the unit left floating is still restricted,
// so an instant (Timely Interference, {U}) cannot be cast with it and the
// opponent's priority menu is pass alone. The kernel once offered the cast
// (decision 380 of tail-23-c001 pair 27).
func TestMBCTimelyInterferenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_leftover_instant")
}
