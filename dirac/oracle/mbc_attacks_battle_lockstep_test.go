package oracle

import "testing"

// CR 508.3a: attacking a battle is not attacking a player or a planeswalker.
// Slumbering Dragon's "Whenever a creature attacks you or a planeswalker you
// control" (Attacked$ You,Planeswalker.YouCtrl) must not fire for a creature
// attacking a battle its controller protects. The kernel once matched any
// permanent the controller controlled, queued a second Dragon trigger and
// asked a trigger_order gorge never posed (tail-12-c000 pair 27, decision 593).
func TestMBCAManticoreReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attacks_battle_is_not_planeswalker")
}
