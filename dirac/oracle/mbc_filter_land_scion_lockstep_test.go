package oracle

import "testing"

// Celestial Archon (and its bestow cost) and Colossal Whale are castable by
// gorge's planner on a board of Artifact Unknown Shores ({1},{T}: one mana of
// any colour) beside an Eldrazi Scion token; the kernel offered neither. The
// recording is the first divergence of the evidence loop's tail-10 pair 139:
// options options at decision 288, gorge leading, on the full surface.
func TestMBCCelestialArchonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_with_scion_cast")
}
