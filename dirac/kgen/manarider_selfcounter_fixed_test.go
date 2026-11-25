package kgen

import (
	"strings"
	"testing"
)

// TestManaRiderSelfCounterFixed: a mana ability whose rider puts a counter on
// its own source lowers on a fixed colour (Vulshok Factory's {R}) and a
// finite Combo (Land Cap's "Add {W} or {U}"), with a plain {T} cost, a
// constant counter count and no condition — the same shape Twitching Doll
// lowers on one mana of any colour (ctrRiderMana); the kernel's counter-rider
// arm (mana_rider.go) is colour-agnostic and gorge's planner defers the
// source off the SubAbility$ chain alone (pay.PaymentPlanAbilityTier). The
// map pins the class over every corpus card whose mana ability's SubAbility$
// is a PutCounter: the fixed-colour and finite Combo forms lower, in
// printed order or not (Hourglass of the Lost's {W}, Strixhaven Stadium's
// {C}, Land Cap's W U and Veldt's G W), the shapes whose mana amount or
// another part reads the counter stay refused with the reason fragment
// named.
func TestManaRiderSelfCounterFixed(t *testing.T) {
	// lowered -> "" (no refusals), refused -> a fragment of its reason.
	want := map[string]string{
		"Land Cap":                "",
		"Vulshok Factory":         "",
		"Hourglass of the Lost":   "",
		"Strixhaven Stadium":      "",
		"Lava Tubes":              "",
		"River Delta":             "",
		"Timberline Ridge":        "",
		"Twitching Doll":          "",
		"Veldt":                   "",
		"Runecarved Obelisk":      "rider program",        // the mana amount is two
		"Trenzalore Clocktower":   "exile-self cost",      // the second ability exiles the source
		"Glittering Stockpile":    "counter rider beside", // the rider stands beside a second mana ability
		"Eumidian Hatchery":       "rider program",        // the cost pays a life
		"Pyramid of the Pantheon": "rider program",        // CheckSVar$ on the mana ability
		"Cryptex":                 "evidence cost",
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		wantReason, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if wantReason == "" {
			if len(c.Refusals) != 0 {
				t.Errorf("%s refused: %v", c.Name, c.Refusals)
				continue
			}
			found := false
			for _, a := range c.Abilities {
				if !ctrRiderMana(a) {
					continue
				}
				found = true
				if a.Mana.Choice != (len(a.Mana.Colors) > 1 && a.Mana.Colors != "WUBRG") {
					t.Errorf("%s: mana choice %v colours %q", c.Name, a.Mana.Choice, a.Mana.Colors)
				}
			}
			if !found {
				t.Errorf("%s: no ability is the counter-rider mana shape", c.Name)
			}
			continue
		}
		matched := false
		for _, r := range c.Refusals {
			matched = matched || strings.Contains(r.Reason, wantReason)
		}
		if !matched {
			t.Errorf("%s: not refused with %q (%v)", c.Name, wantReason, c.Refusals)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
