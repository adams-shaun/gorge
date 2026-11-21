package oracle

import "testing"

// Storm the Seedcore ("distribute four +1/+1 counters among up to four target
// creatures you control") may be cast with no targets; the first target
// decision must offer done on both engines.
func TestMBCStormTheSeedcoreReprosLockstep(t *testing.T) {
	if landGrantReplay(t) {
		return
	}
	stdReproNamed(t, "divided_counters_up_to_zero_targets")
}
