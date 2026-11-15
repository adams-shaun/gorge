package oracle

import "testing"

// Tranquil Frillback's "pay {G} up to three times" announcement: the evidence
// loop's pool tail-35-c005 pair 15 parted at decision 225 (gorge offered the
// counts 0..3, the kernel only the decline).
func TestMBCTranquilFrillbackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "frillback_announce_count_unpayable")
}
