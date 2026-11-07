package oracle

import "testing"

// A Convoke announcement on a cast gorge's offer made through a play
// permission (CR 702.51a, 601.2h) keeps the colour picks its cost names even
// after an earlier pick of that colour paid the pip: the oracle has no
// priority-time cost for a may-play cast, so it does not filter the
// announcement's static option list, and the kernel must pose the same menu.
func TestMayplayConvokeOptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-ecl", []stdRepro{
		{"mayplay_convoke_options", false, false},
	})
}
