package oracle

import (
	"testing"
)

// CR 603.7 with CR 400.7: Lae'zel's Acrobatics rolled 10-20, returned its two
// exiled creatures (one a transformed Ballista Watcher) untapped, exiled them
// again and registered its delayed end-of-turn return over the set the re-exile
// re-remembered — a bounded ChangeZone's RememberChanged$ write, not the
// ChangeZoneAll sweep the 1-9 clause registers over. The kernel lowered that
// second registration to its one-object delayed program, so the trigger
// returned only the set's first object (Stick, Fearless Mentor) and left
// Ballista Watcher in exile (decision 392 of 624; pool tail-19 pair 3).
func TestMBCBallistaWatcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_chain_remembered_set_first_only")
}
