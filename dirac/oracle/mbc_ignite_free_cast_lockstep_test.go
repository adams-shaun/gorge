package oracle

import "testing"

// Ignite the Future cast from a graveyard grants the plain and the free play
// over each card it exiles. An exiled spell both cover (Bushmaster, Coiled
// Henchman) is one free cast at priority, as gorge lists it, not one per
// permission: the cast half of ignite_future_double_play_permission.
func TestMBCIgniteFreeCastLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ignite_future_double_free_cast")
}
