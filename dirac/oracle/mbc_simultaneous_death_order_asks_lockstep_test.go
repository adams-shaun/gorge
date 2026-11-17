package oracle

import "testing"

// CR 616.1 with CR 704.3: a state-based pass that puts several permanents into
// their owners' graveyards poses one replacement-order ask for each, and gorge
// takes them in its pass's family order: the lethal-damage creatures first,
// then the zero-loyalty walkers, then the unattached Auras. The kernel parked
// every graveyard-bound casualty in seat order instead, so a pass that killed a
// creature of each seat and a walker asked the second question of p0 where
// gorge asked p1: `decision kind ref replacement p1, kernel replacement p0`,
// the class the evidence loop reported on Ophidian's game. That game no longer
// replays (Thermopod's by-name refusal re-dealt the dr-mbc-baaa evidence focus);
// this recording is pair 213 of the same focus with Thermopod dropped, whose
// first divergence is the same class (the third question is p0's walker either
// way, so only the order of the asks differs).
func TestMBCOphidianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "simultaneous_death_order_asks")
}
