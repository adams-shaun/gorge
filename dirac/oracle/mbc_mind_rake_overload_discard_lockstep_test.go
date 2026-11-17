package oracle

import "testing"

// CR 101.4 (APNAP order) with CR 702.96a (overload): an overloaded Mind Rake
// is "each player discards two cards", and the caster of a sorcery is the
// active player, so the discard decisions run caster first, then turn order.
// Seat 1 casts it here: gorge asked seat 0 first (its census listed players
// from seat 0), the kernel asked the caster (decision 90, kind ref modes p0
// vs p1; the kernel leads and is correct). The recording is pair 4 of the
// tail-76 focus pool dealt around Mind Rake and Will of the Naga.
func TestMBCMindRakeOverloadDiscardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mind_rake_overload_discard")
}
