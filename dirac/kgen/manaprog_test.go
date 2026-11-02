package kgen

import (
	"strings"
	"testing"
)

// TestManaRiderRefused (kmanaprog): a mana ability whose chain carries
// a rider (CR 605.3b: it resolves with the mana) is refused, since the
// kernel's autopay adds the mana and runs no program -- except a painland's
// "deals N damage to you" (painRider, TestPainlandLowered).
func TestManaRiderRefused(t *testing.T) {
	want := map[string]bool{"Chromatic Sphere": true, "Peat Bog": true,
		"Eumidian Hatchery": true, "Grove of the Burnwillows": true}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		if !want[c.Name] {
			continue
		}
		seen++
		found := false
		for _, r := range c.Refusals {
			found = found || strings.Contains(r.Reason, "mana ability with a rider program")
		}
		if !found {
			t.Errorf("%s: not refused as a mana ability with a rider (%v)", c.Name, c.Refusals)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}

// TestSearchRememberLowered: a library search's RememberChanged$ lowers to
// the Remember flag (kop.SrRemember in the table), not dropped.
func TestSearchRememberLowered(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Fabled Passage", "Rampaging Growth", "Instrument of the Bards"}) {
		if len(c.Refusals) > 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "|Remember") && !strings.Contains(sb.String(), "CS=Remember") {
			t.Errorf("%s: search lacks Remember:\n%s", c.Name, sb.String())
		}
	}
}

// TestPainlandLowered: a painland's {C} beside its painful
// colour choice, and a pain-only source (Elves of Deep Shadow), lower whole;
// a painful ability beside a coloured first one (City of Brass-like shapes
// stay refused elsewhere) is not this shape.
func TestPainlandLowered(t *testing.T) {
	want := map[string]bool{"Adarkar Wastes": true, "Shivan Reef": true, "Elves of Deep Shadow": true}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		if !want[c.Name] {
			continue
		}
		seen++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
