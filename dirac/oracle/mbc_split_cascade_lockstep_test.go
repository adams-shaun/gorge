package oracle

import "testing"

// CR 702.85a with CR 709.4b: cascade compares a split card's mana value as
// the halves' sum once it is in exile (Integrity // Intervention is 6, not
// Integrity's 2), so Into the Time Vortex (5) passes over it and finds Herald
// of the Fair. dirac read the front half alone, exiled Herald and kept
// Integrity as the cascaded card (decision 297, tail-31 pair 83).
func TestMBCIntegrityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "integrity_herald_zone_order")
}
