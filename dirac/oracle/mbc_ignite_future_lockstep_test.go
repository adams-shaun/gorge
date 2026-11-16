package oracle

import "testing"

// Ignite the Future exiles three cards and grants two MayPlay effects: the
// plain one always, the free one only when the spell was cast from a graveyard.
// A Plains exiled by a spell cast from hand must be offered as one land play,
// not once per effect.
func TestMBCIgniteTheFutureReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ignite_future_double_play_permission")
}
