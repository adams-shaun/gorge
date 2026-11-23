package oracle

import "testing"

// Wild Hunger in a graveyard with flashback {3}{R}, after an Equip {3} paid
// from three of the lands: gorge's search still finishes and offers the
// flashback cast, so dirac must offer it too (tail-15, decision 1252).
func TestMBCWildHungerFloatingRReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wild_hunger_flashback_after_equip")
}
