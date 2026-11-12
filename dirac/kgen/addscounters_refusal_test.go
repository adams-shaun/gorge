package kgen

import (
	"strings"
	"testing"
)

// TestAddsCountersManaRiderRefusedByName: the four corpus cards with an
// AddsCounters$ mana rider (Opal Palace, Biophagus, Animal Attendant,
// Guildmages' Forum) stay refused, each with the planner-deferral reason
// (lower_manaspent.go refuseAddsCounters), not a bare "unread params".
func TestAddsCountersManaRiderRefusedByName(t *testing.T) {
	names := []string{"Opal Palace", "Biophagus", "Animal Attendant", "Guildmages' Forum", "Carnelian Orb of Dragonkind"}
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, names) {
		seen[c.Name] = true
		var all []string
		for _, r := range c.Refusals {
			all = append(all, r.Reason)
		}
		got := strings.Join(all, "; ")
		if c.Name == "Carnelian Orb of Dragonkind" {
			// the neighbouring AddsKeywords$ rider is untouched
			if len(c.Refusals) != 0 {
				t.Errorf("%s: refusals %q, want none", c.Name, got)
			}
			continue
		}
		if !strings.Contains(got, "AddsCounters$ mana rider: gorge's planner defers the source") {
			t.Errorf("%s: refusals %q, want the AddsCounters$ planner-deferral reason", c.Name, got)
		}
		if strings.Contains(got, "unread params") {
			t.Errorf("%s: refused as an unread param, not by name: %q", c.Name, got)
		}
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("precondition: %s not lowered (corpus missing?)", n)
		}
	}
}
