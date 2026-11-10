package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestLowerTapsformanaAnycolour: a static TapsForMana trigger adding one mana
// of the type a land produced lowers to a StatTapExtra of one mana with
// StTapReflect (gorge pay.PaymentPlanReflectedTapMana folds it per production),
// StTapAnyPlayer when it reaches every player's taps (Lavaleaper, Mana Flare,
// Heartbeat of Spring, Keeper of Progenitus), not for the controller's own
// (Nikya of the Old Ways). Out-of-shape variants still refuse by name: a
// nonland filter (Kinnan, Roxanne), a reflected body with a rider (Winter's
// Night), and an any-colour extra (Buried in the Garden, lt-low-tapsformana-choice's).
func TestLowerTapsformanaAnycolour(t *testing.T) {
	const anyPlayer, own = "any", "own"
	want := map[string]string{
		"Lavaleaper":               anyPlayer,
		"Mana Flare":               anyPlayer,
		"Heartbeat of Spring":      anyPlayer,
		"Keeper of Progenitus":     anyPlayer,
		"Nikya of the Old Ways":    own,
		"Kinnan, Bonder Prodigy":   "not a land filter",
		"Roxanne, Starfall Savant": "not a land filter",
		"Winter's Night":           "trigger mode TapsForMana",
		"Buried in the Garden":     "trigger mode TapsForMana",
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
		if reason != anyPlayer && reason != own {
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
			if !slices.Contains(st.Flags, "TapReflect") || st.TapAmt != 1 {
				t.Errorf("%s: TapExtra flags %v amount %d, want TapReflect and one mana", c.Name, st.Flags, st.TapAmt)
			}
			if got := slices.Contains(st.Flags, "TapAnyPlayer"); got != (reason == anyPlayer) {
				t.Errorf("%s: TapAnyPlayer = %v, want %v (flags %v)", c.Name, got, reason == anyPlayer, st.Flags)
			}
			if st.F == nil {
				t.Errorf("%s: TapExtra without its land filter", c.Name)
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
