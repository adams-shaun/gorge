package oracle

import "testing"

// Guardian of Faith's enter trigger phased out a Wall of Roots that wore a
// bestowed Nyxborn Hydra and a Darksteel Mutation (the first divergence of
// dr-mbc-bbaa pair 15, land 8bab7eb132). A phased-out permanent keeps its
// counters (CR 702.26d) and its own rule-based type switches (CR 702.114e: an
// attached bestow card is an Aura, not a creature) -- the kernel's projection
// dropped the wall's -0/-1 counter and re-creaturized the Aura into a 1/2.
func TestMBCNyxbornHydraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_phased_out_projection_counters_and_switches")
}
