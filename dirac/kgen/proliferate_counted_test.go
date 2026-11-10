package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const prolifCountedHead = "Name:Probe\nManaCost:1 G\nTypes:Creature Wizard\nPT:1/1\n"

// TestProliferateCountedAmount: an Amount$ naming an SVar count of the
// distinct planeswalker types lowers to Proliferate over that value (the
// ExCount N 1 expression); a paid X and a non-count SVar still refuse.
func TestProliferateCountedAmount(t *testing.T) {
	c := abLower(t, prolifCountedHead+
		"A:AB$ Proliferate | Cost$ W U B R G T | Amount$ ProbeWalkerTypes | SpellDescription$ x\n"+
		"SVar:ProbeWalkerTypes:Count$Valid Planeswalker.YouCtrl$PlaneswalkerType\nOracle:x\n", false)
	if len(c.Refusals) != 0 {
		t.Fatalf("counted proliferate refused: %s", abReasons(c))
	}
	var found bool
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "Proliferate" {
				found = true
				if in.A.Kind != vPWTypes || in.A.F == nil {
					t.Errorf("Proliferate amount = %s, want PWTypes over a filter", in.A)
				}
			}
		}
	}
	if !found {
		t.Fatal("no Proliferate instruction lowered")
	}
	var cb cardBuilder
	cb.pt = &poolTables{exprKey: map[string]int16{}}
	if e, err := cb.expr(lValue{Kind: vPWTypes}); err != nil || e.Kind != kop.ExCount || e.N != 1 {
		t.Errorf("PWTypes builds to %+v, %v; want ExCount N 1", e, err)
	}
	inline := abLower(t, prolifCountedHead+
		"A:AB$ Proliferate | Cost$ 1 | Amount$ Number$2/Minus.ProbeRem | SpellDescription$ x\n"+
		"SVar:ProbeRem:Count$RememberedSize\nOracle:x\n", false)
	if why := abReasons(inline); !strings.Contains(why, "Proliferate Amount$") {
		t.Errorf("inline Number$ amount: refusals %q, want a Proliferate Amount$ refusal", why)
	}
	for amt, svar := range map[string]string{
		"ProbePaid": "Count$xPaid",
		"ProbeAgg":  "Count$Valid Planeswalker.YouCtrl$NoSuchAggregate",
	} {
		r := abLower(t, prolifCountedHead+
			"A:AB$ Proliferate | Cost$ 1 | Amount$ "+amt+" | SpellDescription$ x\n"+
			"SVar:"+amt+":"+svar+"\nOracle:x\n", false)
		if why := abReasons(r); !strings.Contains(why, "Proliferate Amount$") {
			t.Errorf("Amount$ %s=%s: refusals %q, want a Proliferate Amount$ refusal", amt, svar, why)
		}
	}
}
