package oracle

import "testing"

// CR 602.2 with CR 605.3b: Legacy Weapon's {W}{U}{B}{R}{G} activation is offered
// exactly when five lands can pay one colour each. Decision 891 of the tail-61
// evidence pair (focus_00026_WBR vs focus_00389_WBR, seed 340771) had the kernel
// offering activate(Legacy Weapon) while gorge's priority decision held only pass
// and Ogre Menial's ability: at main b4b304271c the two parted at `options options`.
// Both engines now agree on the offer.
func TestMBCLegacyWeaponReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "legacy_weapon_activation_offer")
}
