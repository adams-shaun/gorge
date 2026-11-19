package oracle

import "testing"

// CR 601.2b with CR 601.2g and CR 605.3a: Chord of Calling is announced with
// convoke picks beside Desolate Mire's "{1}, {T}: Add {W}{B}", so each X the
// ask offers is paid through the same window the filter's fee is funded from,
// with the announced picks folded into the cost at that X. gorge's xAsk
// offers X = 4; the kernel's filter bound priced the unfolded cost and
// stopped one short (decision 421, tail-75-c000 pair 7, autopay).
func TestMBCTail75ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xask_convoke_folded_bound")
}
