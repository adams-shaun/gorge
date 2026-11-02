package feat

import "testing"

// TestV2BlockLayout pins the v2 block's slots to policynet's and the pip
// count's edge shapes (generic clamp, twobrid colour half, X, no cost).
func TestV2BlockLayout(t *testing.T) {
	if entColourOff != 84 || entPipsOff != 90 || entCategory != 96 || EntityRawWidth != 100 {
		t.Fatalf("layout moved: %d %d %d w=%d", entColourOff, entPipsOff, entCategory, EntityRawWidth)
	}
	if EntityRawWidthRelEnd != 84 || entCategory-entColourOff != len(card{}.colourPips) {
		t.Fatalf("rel end %d, colour/pip lanes %d", EntityRawWidthRelEnd, len(card{}.colourPips))
	}
	for _, c := range []struct {
		cost string
		want [6]int
	}{
		{"{2}{W}", [6]int{1, 0, 0, 0, 0, 2}},
		{"{G}{G}", [6]int{0, 0, 0, 0, 2, 0}},
		{"{U}{B}", [6]int{0, 1, 1, 0, 0, 0}},
		{"{X}{R}", [6]int{0, 0, 0, 1, 0, 0}},
		{"no cost", [6]int{0, 0, 0, 0, 0, 0}},
		{"", [6]int{0, 0, 0, 0, 0, 0}},
		{"{6}{W}", [6]int{1, 0, 0, 0, 0, 5}}, // generic clamps at pipMax
		{"{2/W}", [6]int{1, 0, 0, 0, 0, 0}},  // twobrid: colour half only
		{"{G/W}", [6]int{1, 0, 0, 0, 1, 0}},  // hybrid: each face
		{"{C}{C}", [6]int{0, 0, 0, 0, 0, 2}}, // colourless pips share the generic lane
	} {
		if got := pipCounts(c.cost); got != c.want {
			t.Errorf("pipCounts(%q) = %v, want %v", c.cost, got, c.want)
		}
	}
	got := v2ColourPips("{1}{W}{W}")
	want := [12]float32{1, 0, 0, 0, 0, 0, 2.0 / 5, 0, 0, 0, 0, 1.0 / 5}
	if got != want {
		t.Errorf("v2ColourPips(1WW) = %v, want %v", got, want)
	}
	if !containsAnyFold("Deals 3 damage", removalAPIMarkers) || containsAnyFold("Draw", removalAPIMarkers) ||
		!containsAnyFold("Arcane Signet", rampNameMarkers) {
		t.Error("marker match")
	}
}
