package kgen

import (
	"strings"
	"testing"
)

// staticZonesOffZoneForms are the staticzones umbrella's off-zone
// parameterized keyword grants that lower: granted Miracle (kop.StatMiracleGrant,
// lower_miraclegrant.go) and the hand Cycling grants (kop.StatCyclingGrant,
// lower_offzone.go offZoneCycling), and the off-zone layer-4 override grants
// (lower_offzoneoverride.go; TestStaticZonesOffZoneOverride).
var staticZonesOffZoneForms = []string{
	"Aminatou, Veil Piercer", "Lorehold, the Historian", "Molecule Man",
	"Homing Sliver", "Jo Grant", "Rhet-Tomb Mystic", "Tectonic Reformation",
}

func TestStaticZonesOffZoneForms(t *testing.T) {
	for _, name := range staticZonesOffZoneForms {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// staticZonesOffZoneDeferred are the rest of the umbrella's fifteen carriers:
// a granted Madness over six zones and the off-zone ability-loss change
// (Swirl the Mists' colour-word change lowers as kop.StatColorWords,
// lower_colorwords.go; Conspiracy, Painter's Servant and Secret Arcade keep the
// named refusals of TestStaticZonesOffZoneOverride). Each keeps its generic AffectedZone refusal until
// its own projection and kernel reader land (the tickets filed with this
// slice); a lowering that lands moves its row into staticZonesOffZoneForms.
var staticZonesOffZoneDeferred = []string{
	"Falkenrath Gorger", "Yixlid Jailer",
}

func TestStaticZonesOffZoneDeferred(t *testing.T) {
	for _, name := range staticZonesOffZoneDeferred {
		c := lowerNamedCard(t, name)
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, "AffectedZone")
		}
		if !hit {
			t.Errorf("%s refusals = %v, want an AffectedZone refusal", name, c.Refusals)
		}
	}
}
