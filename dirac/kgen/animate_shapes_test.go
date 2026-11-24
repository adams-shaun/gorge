package kgen

import "testing"

// animateLandTypeCarriers print an Animate | Types$ <basic land type> |
// RemoveLandTypes$ True ("target land becomes a Swamp"): CR 305.7's land-type
// setter, the override record's SetLandSub beside LoseAbilities, in every
// duration the cards use (until end of turn, Permanent, UntilHostLeavesPlay,
// AsLongAsInPlay).
var animateLandTypeCarriers = []string{
	"Cyclopean Giant", "Cyclopean Titan", "Deepwood Elder", "Dreamwinder", "Floodchaser",
	"Graceful Antelope", "Kavu Recluse", "Slimy Kavu", "Streambed Aquitects", "Thelonite Monk",
	"Tidal Warrior", "Tide Shaper", "Tundra Kavu", "Zombie Trailblazer",
}

func TestAnimateShapesSlice(t *testing.T) {
	for _, name := range animateLandTypeCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// A land-type Animate beside another type change or a lose-all of its own
// stays refused: the record is the layer-4 setter alone.
func TestAnimateLandTypeSetterRefusals(t *testing.T) {
	o := &lOverride{AddSubtypes: []string{"Swamp"}}
	if err := animateLandTypeSetter(o, false); err != nil || !o.SetLandSub || !o.LoseAbilities {
		t.Fatalf("plain setter: err %v, SetLandSub %v, LoseAbilities %v", err, o.SetLandSub, o.LoseAbilities)
	}
	for name, mk := range map[string]func() (*lOverride, bool){
		"remove-all":      func() (*lOverride, bool) { return &lOverride{AddSubtypes: []string{"Swamp"}}, true },
		"added-card-type": func() (*lOverride, bool) { return &lOverride{AddSubtypes: []string{"Swamp"}, AddTypes: 2}, false },
		"nonbasic-word":   func() (*lOverride, bool) { return &lOverride{AddSubtypes: []string{"Desert"}}, false },
		"no-word":         func() (*lOverride, bool) { return &lOverride{}, false },
	} {
		o, ra := mk()
		if animateLandTypeSetter(o, ra) == nil {
			t.Errorf("%s: want a refusal", name)
		}
	}
}
