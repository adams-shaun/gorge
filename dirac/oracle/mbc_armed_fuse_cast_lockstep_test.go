package oracle

import "testing"

// CR 702.102a with CR 601.2c: Armed // Dangerous is a fuse split card, and
// from hand its Armed half ("Target creature gets +1/+1 and gains double
// strike") is castable on its own whenever a creature can be targeted. The
// kernel offered cast(Armed) at decision 116 of 116 (priority, options,
// gorge leading, on the autopay surface; dr-mbc-aabb pair 91) where gorge
// offered only pass and the land.
func TestMBCArmedReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "armed_fuse_cast_offer")
}
