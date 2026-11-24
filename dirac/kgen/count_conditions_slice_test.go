package kgen

import "testing"

// countConditionsLowered are the gap-count-conditions cards this slice
// lowers: the two-colour devotion gods and March of the Canonized
// (Count$DevotionDual, the Devotion head over a union mask), Decommission
// (bare Condition$ Revolt) and Poison the Cup (bare Condition$ Foretold).
var countConditionsLowered = []string{
	"Athreos, God of Passage", "Iroas, God of Victory", "Karametra, God of Harvests",
	"Klothys, God of Destiny", "Mogis, God of Slaughter", "Phenax, God of Deception",
	"Xenagos, God of Revels", "March of the Canonized", "Decommission", "Poison the Cup",
}

func TestCountConditionsSlice(t *testing.T) {
	for _, name := range countConditionsLowered {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// TestDevotionDualNeedsTwoColours: gorge fails a DevotionDual head without
// exactly two readable colours closed, so kgen refuses it rather than lowering
// a one-colour devotion.
func TestDevotionDualNeedsTwoColours(t *testing.T) {
	for _, rest := range []string{"DevotionDual.White", "DevotionDual.White.Blue.Red", "DevotionDual."} {
		if _, ok, err := devotionHead(rest); !ok || err == nil {
			t.Errorf("%s: ok=%v err=%v, want a refusal", rest, ok, err)
		}
	}
	for _, rest := range []string{"DevotionDual.White.Blue", "Devotion.White", "Devotion.White.Blue"} {
		if _, ok, err := devotionHead(rest); !ok || err != nil {
			t.Errorf("%s: ok=%v err=%v, want it lowered", rest, ok, err)
		}
	}
}
