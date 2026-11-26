package kgen

import (
	"strings"
	"testing"
)

// staticZonesOffZoneForms are the staticzones umbrella's off-zone
// parameterized keyword grants that lower: granted Miracle (kop.StatMiracleGrant,
// lower_miraclegrant.go), granted Madness over six zones (kop.StatMadnessGrant,
// lower_madnessgrant.go), the hand Cycling grants (kop.StatCyclingGrant,
// lower_offzone.go offZoneCycling), and the off-zone layer-4 override grants
// (lower_offzoneoverride.go; TestStaticZonesOffZoneOverride).
var staticZonesOffZoneForms = []string{
	"Aminatou, Veil Piercer", "Lorehold, the Historian", "Molecule Man",
	"Homing Sliver", "Jo Grant", "Rhet-Tomb Mystic", "Tectonic Reformation",
	"Falkenrath Gorger",
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
// the off-zone ability-loss change (Swirl the Mists' colour-word change lowers
// as kop.StatColorWords, lower_colorwords.go; Conspiracy, Painter's Servant and
// Secret Arcade keep the named refusals of TestStaticZonesOffZoneOverride;
// Falkenrath Gorger's granted Madness lowers, TestStaticZonesOffZoneMadness).
// Each keeps its generic AffectedZone refusal until
// its own projection and kernel reader land (the tickets filed with this
// slice); a lowering that lands moves its row into staticZonesOffZoneForms.
// Yixlid Jailer is not a row: its static lowers
// (TestStaticZonesOffZoneAbilityLoss) and the card stays refused, by name,
// until dsub-jailer's readers are all wired.
var staticZonesOffZoneDeferred = []string{}

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

// TestStaticZonesOffZoneMadness: Falkenrath Gorger's six-zone Madness grant
// lowers to one MadnessGrant static per listed zone, each over the Vampire
// filter and the card's own mana cost (CardCostLess 1, "reduced by 0").
func TestStaticZonesOffZoneMadness(t *testing.T) {
	c := lowerNamedCard(t, "Falkenrath Gorger")
	for _, rf := range c.Refusals {
		if strings.Contains(rf.Reason, "AffectedZone") {
			t.Errorf("refusal %q, want none naming AffectedZone", rf.Reason)
		}
	}
	want := map[int]bool{int(zHand): true, int(zLibrary): true, int(zGraveyard): true, int(zExile): true, int(zCommand): true, int(zStack): true}
	n := 0
	for _, s := range c.Statics {
		if s.Kind != "MadnessGrant" {
			continue
		}
		n++
		if !want[s.Zone] {
			t.Errorf("static zone %d is not one of the six listed zones (or repeats)", s.Zone)
		}
		delete(want, s.Zone)
		if s.F == nil || len(s.F.Clauses) != 1 || s.F.Clauses[0].Types&tCreature == 0 {
			t.Errorf("zone %d: filter %v, want the Vampire creature filter", s.Zone, s.F)
		}
		if s.CardCostLess != 1 {
			t.Errorf("zone %d: CardCostLess %d, want 1", s.Zone, s.CardCostLess)
		}
	}
	if n != 6 || len(want) != 0 {
		t.Errorf("%d MadnessGrant statics, zones unseen %v; want six, one per listed zone", n, want)
	}
}
