package kgen

import (
	"strings"
	"testing"
)

// TestPlayConditionKeySorted pins the sorted-first key named by the two
// refusal loops that used to iterate a params map. Hidetsugu and Kairi's Play
// carries ConditionCheckSVar and ConditionSVarCompare; Raphael, Most
// Attitude's Effect static Continuous carries MayPlay and ValidSA. Without
// the sorted iteration the named key changed from run to run.
func TestPlayConditionKeySorted(t *testing.T) {
	cardsDir := ratchetCards(t)
	cases := []struct{ card, reason string }{
		{"Hidetsugu and Kairi", "Play ConditionCheckSVar"},
		{"Raphael, Most Attitude", "Effect static Continuous param MayPlay"},
	}
	for _, tc := range cases {
		p, err := loadCardNames([]string{tc.card}, cardsDir)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 20; i++ {
			lw := lowerPool(p)
			var named []string
			for _, c := range lw.cards {
				for _, r := range c.Refusals {
					if strings.Contains(r.Reason, "Play Condition") || strings.Contains(r.Reason, "Continuous param ") {
						named = append(named, r.Reason)
					}
				}
			}
			if len(named) != 1 || !strings.HasSuffix(named[0], tc.reason) {
				t.Fatalf("%s run %d: named refusals = %q, want exactly one ending %q", tc.card, i, named, tc.reason)
			}
		}
	}
}
