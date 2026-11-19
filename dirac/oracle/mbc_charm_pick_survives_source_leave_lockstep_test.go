package oracle

import "testing"

// A restricted charm's pick log survives its source leaving the battlefield.
// Genku, Future Shaper ("whenever another nontoken permanent you control
// leaves the battlefield, choose one that hasn't been chosen this turn") died
// to a mass destruction while two of its triggers were queued; the triggers
// resolved after the leave, and the second offer had to drop the mode the
// first had picked (CR 400.7 makes the leaving permanent a new object, but
// the picks a queued trigger makes afterwards are the resolving ability's own
// memory, on the source object -- Forge logs them on the Card at push, and
// gorge's RecordCharmChoices records on the source object regardless of zone;
// only leaving the battlefield empties the log, engine/events/apply_zone.go).
// The kernel keyed its pick log by battlefield slot, found no slot for the
// dead source and offered all three modes again (tail-09 pair 36, decision
// 736).
func TestMBCGenkuFutureShaperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "charm_pick_survives_source_leave")
}
