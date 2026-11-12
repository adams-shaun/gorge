package kgen

import (
	"strings"
	"testing"
)

// TestForceOfWillAlternativeCost pins the pay-life and exile-from-hand
// alternative cost: Force of Will's "PayLife<1> ExileFromHand<1/Card.Blue+Other>"
// lowers (kop.CostPayLife|CostExileGrave sharing Cost.N 1, ExileHand), and
// Force of Rowan's red twin with it. The same filtered exile with no life
// part (Misdirection, Snapback, Reverent Mantra's white twin) lowers as the
// exile alone; a count the exile does not match (Commandeer's two cards),
// the Shoal's X read off the exiled card's mana value (Announce$ X beside
// ExileFromHand<1/Card.Black+Other+cmcEQX>) or a second graveyard-reading
// part keeps refusing, by name.
func TestForceOfWillAlternativeCost(t *testing.T) {
	cs := lowerNamed(t, []string{
		"Force of Will", "Force of Rowan", "Misdirection", "Commandeer",
		"Snapback", "Reverent Mantra", "Blazing Shoal",
	})
	want := map[string]string{
		"Force of Will":   "",
		"Force of Rowan":  "",
		"Misdirection":    "",
		"Snapback":        "",
		"Reverent Mantra": "",
		"Commandeer":      `exile cost "ExileFromHand<2/Card.Blue+Other>"`,
		"Blazing Shoal":   "static unread params [Announce]",
	}
	seen := 0
	for _, c := range cs {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		got := strings.Join(why, "; ")
		if w == "" && got != "" {
			t.Errorf("%s refused: %s", c.Name, got)
		}
		if w != "" && !strings.Contains(got, w) {
			t.Errorf("%s refusal = %q, want it to contain %q", c.Name, got, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}
