package oracle

import "testing"

// TestMBCAshlingTheExtinguisherReprosLockstep replays the synthetic tail-49
// game the evidence loop recorded: Ashling, the Extinguisher ({2}{B}{B}) is
// castable by p1 through Bog Initiate's "{1}: Add {B}" beside one Swamp, two
// Mountains, a Forest and Dryad Arbor (divergence at decision 452: options
// options, only gorge offered the cast). The second recording is p0's: no black
// source at all but three Islands and Power Depot, so both B pips come from
// Bog Initiate (divergence at decision 389, options options).
func TestMBCAshlingTheExtinguisherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fixed_feeonly_filter_second_pip")
	stdReproNamed(t, "fixed_feeonly_filter_both_pips")
}
