package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
)

// TestFeatTypeKeyLandOrder pins FeatTypeKey's order for the as-enters Type$
// Land ask's 18 options: sorting by the key lists them in gorge's
// TypeChoiceLabels("Land") order (act_chooseland.go landChoice), the five
// basic labels keeping the Basic Land ask's relative order among them.
func TestFeatTypeKeyLandOrder(t *testing.T) {
	labels := effects.TypeChoiceLabels("Land", "", "")
	if !slices.Equal(labels, landChoice[:]) {
		t.Fatalf("precondition: landChoice = %v, want gorge's %v", landChoice, labels)
	}
	s := &State{}
	keys := make(map[string]uint32, len(landChoice))
	for i, h := range landLabelH {
		keys[landChoice[i]] = s.FeatTypeKey(0, int(h))
	}
	for i := 1; i < len(landChoice); i++ {
		if k, prev := keys[landChoice[i]], keys[landChoice[i-1]]; k <= prev {
			t.Errorf("%s sorts after %s (key %d <= %d), want gorge's option order",
				landChoice[i], landChoice[i-1], k, prev)
		}
	}
	for i := 1; i < len(basicLandChoice); i++ {
		if k, prev := keys[basicLandChoice[i]], keys[basicLandChoice[i-1]]; k <= prev {
			t.Errorf("basic land type %s sorts after %s (key %d <= %d), want the basic ask's order",
				basicLandChoice[i], basicLandChoice[i-1], k, prev)
		}
	}
}
