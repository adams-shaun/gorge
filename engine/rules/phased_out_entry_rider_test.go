package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// phasedOutRiderBearCounters seeds Deanna Troi, Psychic Counsel on seat 0's
// battlefield, optionally phases her out, then enters a Grizzly Bears under
// the same controller and returns the +1/+1 counters it came in with.
func phasedOutRiderBearCounters(t *testing.T, phaseOut bool) int {
	t.Helper()
	e, _ := phasesGame(t, 311, "Deanna Troi, Psychic Counsel", "Grizzly Bears")
	deanna := moveSeededCard(t, e, 0, corpusAlternativeCard(t, "Deanna Troi, Psychic Counsel"), state.ZBattlefield)
	if phaseOut {
		e.emit(events.Event{Kind: events.PhaseOut, Obj: deanna, Amount: 1})
	}
	o := e.G.Obj(deanna)
	if o == nil || o.Zone != state.ZBattlefield || o.PhasedOut != phaseOut {
		t.Fatalf("precondition: Deanna zone/phasedOut = %+v, want battlefield/phasedOut=%v", o, phaseOut)
	}
	bears := moveSeededCard(t, e, 0, corpusAlternativeCard(t, "Grizzly Bears"), state.ZBattlefield)
	b := e.G.Obj(bears)
	if b == nil || b.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bears did not enter: %+v", b)
	}
	return int(b.Counter("P1P1"))
}

// TestPhasedOutEntryRiderDoesNotApply: CR 702.26b — a phased-out permanent is
// treated as though it does not exist, so Deanna Troi's "other creatures you
// control enter with an additional +1/+1 counter" does not apply; phased in,
// it does.
func TestPhasedOutEntryRiderDoesNotApply(t *testing.T) {
	t.Parallel()
	if got := phasedOutRiderBearCounters(t, false); got != 1 {
		t.Fatalf("Deanna phased in: Bears entered with %d +1/+1 counters, want 1", got)
	}
	if got := phasedOutRiderBearCounters(t, true); got != 0 {
		t.Fatalf("Deanna phased out: Bears entered with %d +1/+1 counters, want 0", got)
	}
}
