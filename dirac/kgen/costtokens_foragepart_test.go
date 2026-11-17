package kgen

import "testing"

// payInstr's Forage arm is an early success return before the final one-part
// shape check. The cost parser admits a forage beside another token part in a
// printed trigger's own Cost$ window (unlessParse), so the arm's own guard is
// the only place that can refuse the combination; a part it does not name is
// dropped and the card lowers a free payment. These cases pin the class: the
// Mill/Reveal parts costHand now parses (lower_costtokenforms.go) and the
// older resolution tokens the guard also omitted.
func TestForageBesideTokenPartRefuses(t *testing.T) {
	cases := []struct {
		name string
		cost lCost
	}{
		{"Forage+Mill", lCost{Mana: manaCost{None: true}, Forage: &lFilter{}, MillN: 1}},
		{"Forage+Reveal", lCost{Mana: manaCost{None: true}, Forage: &lFilter{}, RevealN: 1, RevealF: &lFilter{}}},
		{"Forage+Draw", lCost{Mana: manaCost{None: true}, Forage: &lFilter{}, DrawN: 1}},
		{"Forage+DamageYou", lCost{Mana: manaCost{None: true}, Forage: &lFilter{}, DamageYouN: 1}},
	}
	for _, tc := range cases {
		lc := &lowerCtx{trigOwnCost: true}
		// The precondition the assertion depends on: the forage and a second
		// payment part are both present.
		if tc.cost.Forage == nil || tc.cost.MillN+tc.cost.RevealN+tc.cost.DrawN+tc.cost.DamageYouN == 0 {
			t.Fatalf("%s: case is missing a part: %+v", tc.name, tc.cost)
		}
		if in, err := lc.payInstr(tc.cost, lRef{}, false); err == nil {
			t.Errorf("%s: payInstr accepted the combined cost as %q, want refusal", tc.name, in.CS)
		} else if err.Error() != "resolution forage cost" {
			t.Errorf("%s: refused for the wrong reason %q", tc.name, err)
		}
	}

	// A lone forage is still the trigger's whole cost (the shape the corpus's
	// plain Cost$ Forage cards ride): the guard must not over-refuse it.
	lc := &lowerCtx{trigOwnCost: true}
	in, err := lc.payInstr(lCost{Mana: manaCost{None: true}, Forage: &lFilter{}}, lRef{}, false)
	if err != nil {
		t.Fatalf("lone forage refused: %v", err)
	}
	if in.CS != "Forage" {
		t.Fatalf("lone forage lowered as %q, want Forage", in.CS)
	}
}
