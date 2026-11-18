package oracle

import "testing"

// CR 106.6, 605.3: Battery Bearer gives each creature its controller has
// "{T}: Add {C}. This mana can't be spent to cast a nonartifact spell", so
// an untapped Aven Wind Mage and Battery Bearer pay the {2} of Spined Thopter
// (an artifact spell) while every land is tapped, and the {U/P} costs 2 life.
// Gorge offers the cast; the kernel's offer left it out (tail-27-c000 pair 4,
// decision 337).
func TestMBCSpinedThopterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "battery_bearer_artifact_only_grant_pays_phyrexian_artifact")
}
