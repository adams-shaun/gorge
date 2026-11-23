package oracle

import "testing"

// Bloodline Culling {1}{B}{B} (pool tail-42, surface full, decision 310):
// player 0's only payment is Forest and Forest funding School of the Unseen's
// {2},{T} any-colour ability, with Akoum Refuge making B and Karplusan Forest
// the generic, so the cast is payable under CR 605.3a and 601.2g-h. One engine
// offered cast(#1) at priority and the other did not.
func TestMBCBloodlineCullingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "bloodline_culling_school_fee_census")
}
