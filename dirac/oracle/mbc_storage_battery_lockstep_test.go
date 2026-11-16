package oracle

import "testing"

// Haunting Hymn ({4}{B}{B}) at its controller's upkeep, with a Forest
// enchanted by Nature's Embrace ("{T}: Add two mana of any one color"), a
// Swamp, two more Forests and Mercadian Bazaar holding two storage counters:
// seven mana, three of it black, so gorge offers the instant. dirac once left
// the cast out (decision 206; tail-66 pair 1004 at land 3ce538a59c).
func TestMBCHauntingHymnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_beside_granted_land_mana")
}
