package oracle

import "testing"

// CR 506.2 / 508.3a: "Whenever a creature attacks you or a planeswalker you
// control" (Slumbering Dragon, Attacked$ You,Planeswalker.YouCtrl) is keyed to
// a player or a planeswalker defender. A battle (Invasion of Azgol) its
// controller owns is neither, so attacking it does not trigger the Dragon.
// Dirac's attackedMatch checked only zone and controller for an object
// defender and stacked the trigger (decision 377, tail-12 pair 170 seed 31337).
func TestAttackedPlaneswalkerNotBattleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attacked_planeswalker_not_battle")
}
