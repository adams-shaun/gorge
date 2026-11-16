package oracle

import "testing"

// Celestial Archon (and its bestow cost) and Colossal Whale are castable by
// gorge's planner on a board of Artifact Unknown Shores ({1},{T}: one mana of
// any colour) beside an Eldrazi Scion token; the kernel offered neither. The
// recording diverged at decision 104: options options, gorge leading, full
// surface. It replays on a custom pool (a table built from its own decks), not
// tail-10: that pool's membership moves each time the evidence loop credits a
// card, and a recording must not depend on it.
func TestMBCCelestialArchonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_with_scion_cast")
}
