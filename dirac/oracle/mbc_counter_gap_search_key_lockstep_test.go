package oracle

import "testing"

// CR 605.3a, 602.2b: Sunhome, Fortress of the Legion's {2}{R}{W},{T} is
// payable only through a scripted prefix: Vivid Meadow's {W} (by removing a
// charge counter, or plainly), a land funding Prismatic Lens's {1},{T} any
// colour for {R}. gorge's prefix search dedups its payer states on every
// permanent's counters as well as the tapped bits and the pool, so Meadow's
// counter-removing {W} is searched beside its plain {W}; the kernel's key left
// the counters out and pruned that branch, finding the script at a node gorge
// reaches only past its 160-node budget (decision 400 of tail-56 pair 117).
func TestMBCSunhomeFortressOfTheLegionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "counter_gap_prefix_search_state_key")
}
