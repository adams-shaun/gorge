package oracle

import "testing"

// CR 613.1f with CR 613.4c: layer 7 reads the finished layer-6 keywords. Pride
// of the Clouds ("+1/+1 for each other creature with flying") counts the
// creatures a continuous effect granted flying, so with two of them and the
// Khopesh it is 5/5 and the other Pride 4/4, and Guiding Bolt ("target creature
// with power 4 or greater") has targets. gorge's full layer walk counted only
// printed flyers (Derived power 3 and 2 where Power read 5 and 4), so its
// target census was empty and only the kernel offered the cast (decision 381
// of 381; tail-20-c000 pair 5 before the walk read the granted keywords).
func TestMBCGuidingBoltReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "layer7_count_reads_granted_keywords")
}
