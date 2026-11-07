package oracle

import "testing"

// A Convoke announcement on a cast gorge's offer made through a play
// permission (CR 702.51a, 601.2h) poses the menu both engines agree on: the
// oracle prices the may-play cast beside the plain hand offers
// (MayPlayCastPaymentCost) and filters the announcement's static option list
// by it, and the kernel's convoke menu filters the same way. The recorded
// picks never exceed the pips, so the filtered menu still admits them.
func TestMayplayConvokeOptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-ecl", []stdRepro{
		{"mayplay_convoke_options", false, false},
	})
}
