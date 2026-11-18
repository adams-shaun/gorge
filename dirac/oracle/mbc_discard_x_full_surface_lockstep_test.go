package oracle

import "testing"

// Nostalgic Dreams ({G}{G}, "As an additional cost to cast this spell, discard
// X cards") on the full surface: gorge's cast planner never prices an announced
// Discard<X/Spec> ("shape:additional_cost"), and its priority list omitted the
// cast where the kernel offered it (decision 107 of pair 8, pool tail-42-c000,
// surface full, mulligans 0, seed 834125; class options options).
func TestMBCNostalgicDreamsFullSurfaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_x_cast_unpriced_stays_off_list")
}

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
