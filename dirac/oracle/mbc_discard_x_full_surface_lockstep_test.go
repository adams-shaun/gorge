package oracle

import "testing"

// CR 601.2b-c, 106.4: an announced Discard<X> additional cost (Restless
// Dreams) is declined by gorge's cast planner and by its potential planner
// alike, so on the full surface the cast is offered only while the floating
// pool pays it. The kernel lowered a witness that taps lands first (here
// Tendo Ice Bridge's counter mana) and offered the cast with an empty pool
// (tail-01-c000 pair 44, decision 143).
func TestMBCRestlessDreamsFullSurfaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_x_announce_full_surface")
}
