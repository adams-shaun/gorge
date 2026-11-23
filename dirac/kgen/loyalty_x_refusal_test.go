package kgen

import (
	"strings"
	"testing"
)

// TestDynamicLoyaltyRefusedByName: a planeswalker whose printed loyalty is the
// dynamic X form (Nissa, Steward of Elements; Personal Decoy) is refused with
// the loyalty reason. gorge grants such a walker no loyalty counters and
// keeps it on the battlefield; the kernel's CR 704.5i sweep would bin it.
func TestDynamicLoyaltyRefusedByName(t *testing.T) {
	names := []string{"Nissa, Steward of Elements", "Personal Decoy"}
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, names) {
		seen[c.Name] = true
		found := false
		for _, r := range c.Refusals {
			if r.Node == "Loyalty" && strings.Contains(r.Reason, "dynamic starting loyalty") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: refusals %+v, want the dynamic starting loyalty refusal", c.Name, c.Refusals)
		}
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("precondition: %s not lowered (corpus missing?)", n)
		}
	}
}
