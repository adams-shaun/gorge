package kgen

import (
	"strings"
	"testing"
)

// TestPlayPopCountShapes: the Play Amount$/Defined$ population shapes
// (lower_playpopcount.go). The literal Amount$ 2 over Plargg and Nassari's
// Defined$ Remembered.!ChosenCard and Harness the Storm's Play
// Defined$ Targeted lower whole. So do the counted Amount$ shapes: Chandra
// Ablaze's Amount$ ChandraX, the graveyard count of red instants and
// sorceries you own, sizes the one ask's Max over the ValidZone$ Graveyard
// filter population, and Talent of the Telepath's Amount$ X, the
// spell-mastery Count$Compare, sizes it over the remembered library. Each
// lowers onto CastFree's UpTo bit with the bound as a value, not a constant
// (gorge effPlay prices Amount$ through NumResolved and asks with that Max:
// engine/effects/play.go).
//
// Every other census carrier (gap-play-popcounts-shapes) stays refused by
// its other blocker only: the shape arms add no Play Amount/Play Defined$
// refusal to them. Ashiok, Nightmare Muse keeps the exile-filter-population
// literal Amount$ 3, the class gap-play-shapes' exile-population slice owns.
func TestPlayPopCountShapes(t *testing.T) {
	for _, name := range []string{"Plargg and Nassari", "Harness the Storm"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	for _, tc := range []struct {
		name string
		zone int
		kind valKind
	}{
		{"Chandra Ablaze", zGraveyard, vCountGraveyard}, // ChandraX: Count$ValidGraveyard
	} {
		c := lowerNamedCard(t, tc.name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", tc.name, c.Refusals)
			continue
		}
		n := 0
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op != "CastFree" || !strings.Contains(in.CS, "UpTo") {
					continue
				}
				n++
				if in.C != tc.zone {
					t.Errorf("%s: UpTo CastFree zone %d, want %d", tc.name, in.C, tc.zone)
				}
				if in.A.Kind != tc.kind {
					t.Errorf("%s: UpTo bound kind %v, want %v (the ask's Max is the SVar's count)", tc.name, in.A.Kind, tc.kind)
				}
			}
		}
		if n != 1 {
			t.Errorf("%s: %d UpTo CastFree instructions, want 1", tc.name, n)
		}
	}
	// Talent of the Telepath's Play lowers (Amount$ X over the remembered
	// library); its ChangeZone to the graveyard is another blocker, so the
	// card stays refused by that one only.
	for _, r := range lowerNamedCard(t, "Talent of the Telepath").Refusals {
		if strings.Contains(r.Reason, "Play") {
			t.Errorf("Talent of the Telepath: %s survives, want the ChangeZone blocker only", r.Reason)
		}
	}
	// The rest of the census carriers: the accepted shapes add no refusal,
	// so no Play Amount/Play Defined$ survives outside Ashiok's out-of-class
	// exile-population literal.
	for _, name := range []string{
		"Ashiok, Nightmare Muse", "Power Without Equal", "Wand of Wonder",
		"Audacious Swap", "Descendants' Path", "Galvanoth", "Knowledge Pool",
		"Leaf-Crowned Elder", "Maelstrom Archangel Avatar", "Mirrored Depths",
		"Pursued by Something", "Rashmi, Eternities Crafter", "Swindler's Scheme",
		"The Heron Moon", "Unexpected Results", "Yennett, Cryptic Sovereign",
	} {
		for _, r := range lowerNamedCard(t, name).Refusals {
			s := r.Reason
			if !strings.Contains(s, "Play Amount") && !strings.Contains(s, "Play Defined$") {
				continue
			}
			if name == "Ashiok, Nightmare Muse" && strings.HasSuffix(s, `Play Amount "3"`) {
				continue // the exile-filter-population class, gap-play-shapes'
			}
			t.Errorf("%s: %s survives, want refused by its other blocker only", name, s)
		}
	}
}
