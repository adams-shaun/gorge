package oracle

import "testing"

// CR 508.1c with CR 508.1b: a CantAttack restriction's Target$ scope names
// the attacked target, and a battle is neither the static's controller
// ("you") nor that controller's planeswalker. The kernel once folded the
// scope into the creature's filter (every clause +OppCtrl) and dropped the
// creature from the attackers decision entirely, withholding the battle
// pairs a Rite of the Raging Storm's CantAttack (Target$ You,
// Planeswalker.YouCtrl) leaves its Lightning Rager against the controller's
// Siege, Invasion of Moag -- and, on the mirrored read, posing an attackers
// decision whose offer list gorge poses none of (askAttackers' empty option
// list is the silent empty declaration).
func TestMBCInvasionOfMoagCantAttackBattleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cantattack_target_battle_pairs")
}
