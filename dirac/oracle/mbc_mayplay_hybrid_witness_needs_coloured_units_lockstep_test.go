package oracle

import "testing"

// A card with a hybrid-only cost ({W/U}{W/U}{W/U}{W/U}) cast from exile under
// a MayPlayIgnoreColor$ grant (p0's Arcanist's Owl, exiled by p1's
// Brainstealer Dragon) must still be offered when the board's coloured units
// span several colours: gorge's planner folded the four pips to generic, took
// a plan with two colourless units in it, and its own witness (any colour
// never includes colourless, CR 107.4c) refused it, so the cast vanished from
// gorge's menu while the kernel offered it. A colour unit of any colour pays
// each pip under the rider, so the plan has to fund four coloured units
// (Island, two Swamps, Mountain), not four units of any mana.
func TestMBCArcanistSOwlReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_hybrid_witness_needs_coloured_units")
}
