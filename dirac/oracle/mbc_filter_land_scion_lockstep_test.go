package oracle

import "testing"

// Celestial Archon (and its bestow cost) and Colossal Whale are castable by
// gorge's planner on a board of Artifact Unknown Shores ({1},{T}: one mana of
// any colour) beside an Eldrazi Scion token; the kernel offered neither. The
// recording is a small tail-10 deal (decks of a dozen distinct cards, every one
// in the pool) that diverged at decision 104: options options, gorge leading,
// full surface. The loop's own pair 139 held a card the pool later dropped
// (A-Druidic Ritual), so it was not kept.
func TestMBCCelestialArchonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_with_scion_cast")
}
