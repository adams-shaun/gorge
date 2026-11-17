package oracle

import "testing"

// Shilgengar, Sire of Famine returns each creature card from its controller's
// graveyard to the battlefield with a finality counter on it. When one of those
// creatures is sacrificed, CR 122.1h exiles it instead of putting it into the
// graveyard. The kernel's finality check (dieZone) is armed by prepFinality
// from a table scan, which did not see a ChangeZoneAll sweep's WithCountersType$
// rider, so the sacrificed creature went to the graveyard (decision 1032 of the
// recording; tail-21 pair 5, surface autopay).
func TestMBCStingingScorpionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "sweep_finality_counter_exiles_on_death")
}
