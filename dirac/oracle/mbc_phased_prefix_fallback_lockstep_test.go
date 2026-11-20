package oracle

import "testing"

// CR 601.2b / 605.3a: Reclaim the Wastes' kicked cast beside Signpost
// Scarecrow's repeatable "{2}: Add one mana of any color", Plains and Blighted
// Gorge (tail-45 focus pair 22, decision 245 of the recording). gorge's plain
// prefix search budget-limited on the filter's own colour branches and its
// phased fallback (the free gaps, the funding lands, then the fee-carrying
// gap) priced the cast; the kernel's plain walk alone withheld it. The kernel
// now runs the same phased order where its plain run exhausted the budget on a
// fee-only filter board (filtSearch.visit's phased passes), so both engines
// offer the cast and the game stays in lockstep.
func TestMBPhasedReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_prefix_fallback_kicked_cast")
}
