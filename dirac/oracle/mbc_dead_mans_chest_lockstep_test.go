package oracle

import "testing"

// CR 601.2 with Dead Man's Chest: the Chest's exiled cards stay castable for as
// long as they remain exiled (a permanent Effect, no ForgetOnMoved$). Gorge
// offers a Silver Myr the Chest exiled; dirac lowered the grant to an emblem
// whose Card.IsRemembered filter read the cleared remembered set, so the cast
// was never offered (tail-47 pair 25, decision 684).
func TestMBCDaybreakCombatantsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_grant_without_forget_on_moved")
}
