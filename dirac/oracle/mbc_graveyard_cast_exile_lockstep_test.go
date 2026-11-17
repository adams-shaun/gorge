package oracle

import "testing"

// CR 601.2h with CR 118.8: Skaab Ruinator's additional cost exiles three
// creature cards from its controller's graveyard, and a card cast from the
// graveyard has already moved to the stack, so it can never pay its own cost.
// With only two other creature cards beside two Ruinators in the graveyard the
// cast is not offered; the kernel offered both Ruinators (decision 1131 of
// tail-21 pair 1, autopay).
func TestMBCSkaabRuinatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "graveyard_cast_exile_cost_excludes_the_cast_card")
}
