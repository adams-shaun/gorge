package oracle

import "testing"

// Into Thin Air's affinity for artifacts (CR 702.41a) reads the artifacts its
// caster controls, and Goldspan Dragon's granted Treasure ability ({T},
// Sacrifice this artifact: Add two mana of any one color) is a gap in gorge's
// payment census: gorge's prefix search activates it before the cast and
// prices the cast again at every node, so every Treasure it sacrifices is one
// artifact less off the price. Two Treasures and a Bobblehead price the
// spell at {2}{U}, but the prefix that sacrifices both prices it at {4}{U}
// against the four mana it floated, so gorge offers no cast (pool tail-32
// pair 13, decision 448). The kernel priced the cast once, at the root.
func TestMBCIntoThinAirReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "affinity_granted_sac_reprices")
}
