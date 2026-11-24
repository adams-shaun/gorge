package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestLowerTapsformanaChoice: Roxanne, Starfall Savant's static TapsForMana
// extra, one mana of the type an artifact token produced, lowers to a
// StatTapExtra with StTapReflect over the token filter (gorge
// pay.PaymentPlanReflectedTapMana folds it per colour a token's tap makes; the
// kernel prices it on the pick, dirac/mana_tapextra.go). Out-of-shape variants
// still refuse by name: an extra of the colour chosen on an Aura, which also
// needs its SetColor lowered (Shimmerwilds Growth), an extra of any colour
// (Buried in the Garden) lowers as StTapAnyCol, and Regal Behemoth's monarch
// gate refuses by name. A reflected body over a land lowers, as
// TestLowerTapsformanaAnycolour asserts.
func TestLowerTapsformanaChoice(t *testing.T) {
	want := map[string][]string{
		"Roxanne, Starfall Savant": nil,
		"Shimmerwilds Growth":      {`SetColor "ChosenColor"`},
		"Buried in the Garden":     {"anycol"},
		"Regal Behemoth":           {"CheckDefinedPlayer"},
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	slices.Sort(names)
	seen := 0
	for _, c := range lowerNamed(t, names) {
		reasons, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(reasons) == 1 && reasons[0] == "anycol" {
			if len(c.Refusals) != 0 {
				t.Errorf("%s refused: %v", c.Name, c.Refusals)
			}
			n := 0
			for _, st := range c.Statics {
				if st.Kind != "TapExtra" {
					continue
				}
				n++
				if !slices.Contains(st.Flags, "TapAnyCol") || slices.Contains(st.Flags, "TapReflect") {
					t.Errorf("%s: TapExtra flags %v, want TapAnyCol alone", c.Name, st.Flags)
				}
			}
			if n != 1 {
				t.Errorf("%s: %d TapExtra statics, want 1", c.Name, n)
			}
			continue
		}
		for _, reason := range reasons {
			refused := false
			for _, r := range c.Refusals {
				refused = refused || strings.Contains(r.Reason, reason)
			}
			if !refused {
				t.Errorf("%s: not refused with %q (%v)", c.Name, reason, c.Refusals)
			}
		}
		if len(reasons) != 0 {
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
			if !slices.Contains(st.Flags, "TapReflect") || slices.Contains(st.Flags, "TapAnyPlayer") || st.TapAmt != 1 {
				t.Errorf("%s: TapExtra flags %v amount %d, want TapReflect alone and one mana", c.Name, st.Flags, st.TapAmt)
			}
			if st.F == nil {
				t.Errorf("%s: TapExtra without its artifact-token filter", c.Name)
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
