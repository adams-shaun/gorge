package oracle

import "testing"

// TestMBCBeastsOfBogardanReprosLockstep: a Backup-granted dies trigger on a red
// creature targets "creature you control"; the trigger's source is the host
// (red), so a creature with protection from red is not a legal target (CR
// 702.16b, 113.8). dirac took the granter's colours (white) off the pending entry.
func TestMBCBeastsOfBogardanReprosLockstep(t *testing.T) {
	stdReproNamed(t, "granted_trigger_source_protection")
}
