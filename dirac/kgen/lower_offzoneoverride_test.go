package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStaticZonesOffZoneOverride: Maskwood Nexus (AffectedZone$ All) and
// Rootpath Purifier (Battlefield,Library) lower as a plain battlefield
// StatOverride and a StatOffZoneGrant carrying the same override record;
// Conspiracy, Painter's Servant and Secret Arcade keep named refusals.
func TestStaticZonesOffZoneOverride(t *testing.T) {
	for _, tc := range []struct {
		name string
		zone kop.Zone
	}{
		{"Maskwood Nexus", kop.ZoneAny},
		{"Rootpath Purifier", kop.ZoneLibrary},
	} {
		c := lowerNamedCard(t, tc.name)
		if len(c.Refusals) != 0 {
			t.Fatalf("%s refusals = %v, want none", tc.name, c.Refusals)
		}
		var bf, off *lStatic
		for i := range c.Statics {
			s := &c.Statics[i]
			if s.Kind != "Override" {
				continue
			}
			if s.Zone == 0 {
				bf = s
			} else {
				off = s
			}
		}
		if bf == nil || off == nil {
			t.Fatalf("%s statics = %+v, want a battlefield and an off-zone override", tc.name, c.Statics)
		}
		if got := off.Zone; got != int(tc.zone) {
			t.Errorf("%s off-zone static zone = %d, want %d", tc.name, got, tc.zone)
		}
		for _, f := range bf.Flags {
			t.Errorf("%s battlefield half carries flag %q", tc.name, f)
		}
		if len(off.Flags) != 1 || off.Flags[0] != offZoneFlag {
			t.Errorf("%s off-zone half flags = %v, want [%s]", tc.name, off.Flags, offZoneFlag)
		}
	}
	for name, want := range map[string]string{
		"Conspiracy":        refuseOffZoneChosenRemove,
		"Painter's Servant": refuseOffZoneColour,
		"Secret Arcade":     refuseOffZoneStackType,
	} {
		c := lowerNamedCard(t, name)
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, want)
		}
		if !hit {
			t.Errorf("%s refusals = %v, want %q", name, c.Refusals, want)
		}
	}
}
