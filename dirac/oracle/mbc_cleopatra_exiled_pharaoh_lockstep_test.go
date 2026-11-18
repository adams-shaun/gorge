package oracle

import "testing"

// Cleopatra, Exiled Pharaoh ({2}{B}{G}) and Tawnos's Tinkering ({3}{G}) are
// both castable at decision 80 of this recorded game (kfuzz -synth tail-74
// focus_00381_BG vs focus_00383_WB pair 6) only through Lotus Blossom's
// "{T}, Sacrifice this: Add one mana of any color", which the planner's
// census does not price. gorge's offer reaches them through the counters
// gap's prefix script, so the source is tapped and sacrificed at priority
// while the printed sources pay the rest at the cast's mana step. The kernel
// once offered only pass (no counter-cost gap shape), and, with the plain
// offer census widened instead, tapped every printed source at priority
// (decision 85). The recording pins both decisions.
func TestMBCCleopatraExiledPharaohReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cleopatra_exiled_pharaoh")
}
