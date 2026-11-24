package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// TestChooseLandLabels pins the as-enters Type$ Land ask's option encoding:
// landChoice is gorge's TypeChoiceLabels("Land") in its order, the 18 option
// values are pairwise distinct (a collision would answer two labels with one
// type) and clear of the creature-type vocabulary's indexes, and the five
// basic labels keep the basic ask's values.
func TestChooseLandLabels(t *testing.T) {
	want := effects.TypeChoiceLabels("Land", "", "")
	if len(want) != 18 {
		t.Fatalf("precondition: gorge offers %d land types, want 18", len(want))
	}
	if !slices.Equal(want, landChoice[:]) {
		t.Fatalf("landChoice = %v, want gorge's %v", landChoice, want)
	}
	seen := map[uint16]string{}
	for i, h := range landLabelH {
		if h < 0x8000 {
			t.Errorf("%s: option value %#x is not in the label space", landChoice[i], h)
		}
		if prev, dup := seen[h]; dup {
			t.Errorf("%s and %s share option value %#x", prev, landChoice[i], h)
		}
		seen[h] = landChoice[i]
	}
	for i, b := range basicLandChoice {
		if h := labelHash(b); h != basicLandLabelH[i] {
			t.Errorf("%s: labelHash %#x != basicLandLabelH %#x", b, h, basicLandLabelH[i])
		}
		if !slices.Contains(landChoice[:], b) {
			t.Errorf("basic land type %s is not an option of the land ask", b)
		}
	}
	// the decoder maps a label's value to the matchup subtype index, and a
	// type the matchup lacks to 255
	k := &prep{t: &kop.Table{Subtypes: []string{"Aura", "Swamp", "Town"}}}
	for label, want := range map[string]uint8{"Swamp": 1, "Town": 2, "Forest": 255, "Cave": 255} {
		if got := landValue(k, labelHash(label)); got != want {
			t.Errorf("landValue(%s) = %d, want %d", label, got, want)
		}
	}
	if got := landValue(k, 0x1234); got != 255 {
		t.Errorf("landValue(non-label) = %d, want 255", got)
	}
}
