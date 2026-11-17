package kgen

import "testing"

// TestUnlessTokenCombosRefuse pins that an UnlessCost$ combining two
// resolution-time token parts is refused at payInstr's final one-part shape
// check, not silently lowered with one part dropped. resTokenInstr is a
// first-match switch (Draw, DamageYou, Mill, Reveal), so when a Draw/DamageYou
// arm wins, a Mill/Reveal part must survive to the final check; likewise a
// Mill before Reveal. No corpus card carries the combination, so this is a
// unit-level guard over the shape.
func TestUnlessTokenCombosRefuse(t *testing.T) {
	lc := &lowerCtx{unlessCost: true}
	combo := []struct {
		name string
		cost lCost
	}{
		{"Draw+Mill", lCost{Mana: manaCost{None: true}, DrawN: 1, MillN: 1}},
		{"DamageYou+Mill", lCost{Mana: manaCost{None: true}, DamageYouN: 1, MillN: 1}},
		{"Mill+Reveal", lCost{Mana: manaCost{None: true}, MillN: 1, RevealN: 1}},
		{"Draw+Reveal", lCost{Mana: manaCost{None: true}, DrawN: 1, RevealN: 1}},
	}
	for _, tc := range combo {
		// The precondition the assertion depends on: both parts are present.
		parts := 0
		for _, on := range []bool{tc.cost.DrawN > 0, tc.cost.DamageYouN > 0, tc.cost.MillN > 0, tc.cost.RevealN > 0} {
			if on {
				parts++
			}
		}
		if parts != 2 {
			t.Fatalf("%s: case carries %d parts, want 2: %+v", tc.name, parts, tc.cost)
		}
		_, err := lc.payInstr(tc.cost, lRef{}, false)
		if err == nil {
			t.Errorf("%s: payInstr accepted the combined cost, want refusal", tc.name)
			continue
		}
		if err.Error() != "resolution cost shape" {
			t.Errorf("%s: refused for the wrong reason %q, want the final shape check", tc.name, err)
		}
	}

	// A lone part is still the one paid instruction: the final check must not
	// refuse it (this is the shape the landed unless carriers ride).
	single := []struct {
		name string
		cost lCost
	}{
		{"Draw", lCost{Mana: manaCost{None: true}, DrawN: 1}},
		{"Mill", lCost{Mana: manaCost{None: true}, MillN: 1}},
		{"Reveal", lCost{Mana: manaCost{None: true}, RevealN: 1}},
	}
	for _, tc := range single {
		if _, err := lc.payInstr(tc.cost, lRef{}, false); err != nil {
			t.Errorf("%s: payInstr refused the lone part: %v", tc.name, err)
		}
	}
}
