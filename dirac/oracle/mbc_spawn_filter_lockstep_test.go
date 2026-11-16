package oracle

import "testing"

// CR 605.3a: an Eldrazi Scion's "Sacrifice this creature: Add {C}" is priced by
// gorge's planner as a last-resort alternative, not a census gap, so a
// Shimmering Grotto beside it is still a filter-only board. The kernel counted
// the Scion as a gap and offered neither of Nightscape Apprentice's
// "{U}, {T}" and "{R}, {T}" activations (only Grotto makes U or R) where gorge
// offered both (decision 452 of 452; tail-10 pair 792).
func TestMBCNightscapeApprenticeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spawn_beside_filter_is_no_census_gap")
}
