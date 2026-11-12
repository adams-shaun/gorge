package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestLowerTapsformanaAttached: an Aura-on-land mana trigger
// (Mode$ TapsForMana | Static$ True, ValidCard$ Card.AttachedBy, body adding
// the tapped land's controller an extra) lowers to a TapExtra static over the
// attached permanent when the body is one literal colour (Wild Growth,
// Overgrowth, Wolfwillow Haven); StTapAnyPlayer reaches every player's tap of
// the land, since the tapped permanent's controller is the payer. A non-aura
// literal body (Vernal Bloom's Forest, Gauntlet of Might's Mountain) lowers
// the same way. A body whose colour is not literal keeps refusing with its
// own reason: an any-colour body asks a colour choice at tap time (Fertile
// Ground, Verdant Haven, Market Festival, Dawn's Reflection, Buried in the
// Garden, Blighted Burgeoning, Trace of Abundance, Glittering Frost), and a
// chosen-colour body is a source gorge's planner defers (Utopia Sprawl).
// Gauntlet of Power's chosen-colour tap gate is a different shape and still
// refuses generically.
func TestLowerTapsformanaAttached(t *testing.T) {
	type want struct {
		amount   uint8
		attached bool
	}
	lowered := map[string]want{
		"Wild Growth":       {1, true},
		"Overgrowth":        {2, true},
		"Wolfwillow Haven":  {1, true},
		"Vernal Bloom":      {1, false},
		"Gauntlet of Might": {1, false},
	}
	refused := map[string]string{
		"Fertile Ground":       "extra of any colour",
		"Verdant Haven":        "extra of any colour",
		"Market Festival":      "extra of any colour",
		"Dawn's Reflection":    "extra of any colour",
		"Buried in the Garden": "extra of any colour",
		"Blighted Burgeoning":  "extra of any colour",
		"Trace of Abundance":   "extra of any colour",
		"Glittering Frost":     "extra of any colour",
		"Gauntlet of Power":    "trigger mode TapsForMana",
	}
	var names []string
	for n := range lowered {
		names = append(names, n)
	}
	for n := range refused {
		names = append(names, n)
	}
	cards := map[string]*lCard{}
	for _, c := range lowerNamed(t, names) {
		cards[c.Name] = c
	}
	for name, want := range lowered {
		c := cards[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", name, c.Refusals)
			continue
		}
		n := 0
		for _, st := range c.Statics {
			if st.Kind != "TapExtra" {
				continue
			}
			n++
			if !slices.Contains(st.Flags, "TapAnyPlayer") {
				t.Errorf("%s: TapExtra flags %v, want TapAnyPlayer", name, st.Flags)
			}
			if st.TapAmt != want.amount {
				t.Errorf("%s: TapExtra amount %d, want %d", name, st.TapAmt, want.amount)
			}
			if st.F == nil {
				t.Errorf("%s: TapExtra without its filter", name)
			} else if want.attached && (len(st.F.Clauses) != 1 || st.F.Clauses[0].Flags&(fEquippedBy|fEnchantedBy) == 0) {
				t.Errorf("%s: TapExtra filter %v, want one AttachedBy/EnchantedBy clause", name, st.F)
			}
		}
		if n != 1 {
			t.Errorf("%s: %d TapExtra statics, want 1", name, n)
		}
	}
	for name, reason := range refused {
		c := cards[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		found := false
		for _, r := range c.Refusals {
			found = found || strings.Contains(r.Reason, reason)
		}
		if !found {
			t.Errorf("%s: not refused with %q (%v)", name, reason, c.Refusals)
		}
	}
}
