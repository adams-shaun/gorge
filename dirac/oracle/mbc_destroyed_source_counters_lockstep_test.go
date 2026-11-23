package oracle

import "testing"

// CR 608.2h: Voodoo Doll's end-step trigger destroys the Doll, then deals
// damage to its controller equal to the pin counters it had. The count reads
// the Doll's last known counters on the battlefield (3), not the graveyard
// card, whose counters left with it (0).
func TestMBCTail05DestroyedSourceCountersLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "destroyed_source_reads_last_known_counters")
}
