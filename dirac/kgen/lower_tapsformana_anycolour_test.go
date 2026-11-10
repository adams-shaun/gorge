package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestLowerTapsformanaAnycolour: a static TapsForMana trigger adding one mana
// of the type a basic land produced, to whoever tapped it (Lavaleaper),
// lowers to a StatTapExtra of one mana with StTapReflect and StTapAnyPlayer
// (gorge pay.PaymentPlanReflectedTapMana folds it). Out-of-shape variants
// still refuse by name: a reflected body over every land, whose sources may
// make several types (Mana Flare, Nikya of the Old Ways), a reflected body
// with a rider (Winter's Night), and an any-colour extra (Buried in the
// Garden, lt-low-tapsformana-choice's).
func TestLowerTapsformanaAnycolour(t *testing.T) {
	want := map[string]string{
		"Lavaleaper":            "",
		"Mana Flare":            `TapsForMana reflected mana over ValidCard$ "Land"`,
		"Nikya of the Old Ways": `TapsForMana reflected mana over ValidCard$ "Land"`,
		"Winter's Night":        "trigger mode TapsForMana",
		"Buried in the Garden":  "trigger mode TapsForMana",
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	slices.Sort(names)
	seen := 0
	for _, c := range lowerNamed(t, names) {
		reason, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if reason != "" {
			refused := false
			for _, r := range c.Refusals {
				refused = refused || strings.Contains(r.Reason, reason)
			}
			if !refused {
				t.Errorf("%s: not refused with %q (%v)", c.Name, reason, c.Refusals)
			}
			continue
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
		}
		n := 0
		for _, st := range c.Statics {
			if st.Kind != "TapExtra" {
				continue
			}
			n++
			if !slices.Contains(st.Flags, "TapReflect") || !slices.Contains(st.Flags, "TapAnyPlayer") || st.TapAmt != 1 {
				t.Errorf("%s: TapExtra flags %v amount %d, want TapReflect, TapAnyPlayer and one mana", c.Name, st.Flags, st.TapAmt)
			}
			if st.F == nil {
				t.Errorf("%s: TapExtra without its basic-land filter", c.Name)
			}
		}
		if n != 1 {
			t.Errorf("%s: %d TapExtra statics, want 1", c.Name, n)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
