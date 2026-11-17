package oracle

import "testing"

// Dead Man's Chest's dies trigger exiles cards equal to the enchanted
// creature's power from its owner's library and registers a permanent
// Effect over the remembered set whose static is "you may cast [them] as
// long as they remain exiled, mana of any type" -- with no ForgetOnMoved$,
// so the Effect keeps its memory after the following ClearRemembered$
// cleanup. kgen lowered that Effect to an emblem whose MayPlayFrom static
// still carries the IsRemembered filter, but no capture: the filter read
// the resolution set the cleanup had emptied, and the kernel offered no
// cast of the exiled card (decision 423; tail-06 pair 43, the game the
// evidence loop filed under Lagonna-Band Elder / Dead Man's Chest).
func TestMBCDeadManSChestReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "effect_mayplay_remembered_without_forget_on_moved")
}
