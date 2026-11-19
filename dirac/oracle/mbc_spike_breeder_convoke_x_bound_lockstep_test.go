package oracle

import "testing"

// CR 601.2b with CR 702.51a: p0 casts Chord of Calling ({X}{G}{G}{G}) and
// convokes Spike Breeder, a 3/3 carrying three +1/+1 counters. gorge's xAsk
// prices each announced X through paymentManaX, whose fold applies the
// announced convoke picks, so it offered X = 1..3. The kernel's X-ask bound
// priced the raw cost without the announced picks and stopped at X = 2
// (tail-75 pair 18, decision 796, options).
func TestMBCSpikeBreederReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spike_breeder_convoke_x_bound")
}
