package kgen

import (
	"strings"
	"testing"
)

// TestOverloadRememberLKIRefusedByName: Winds of Abandon's overloaded Exile is
// a RememberLKI$ move (lower_zone.go appends |LKI to CS). A ChangeZoneAll
// sweep has no per-object controller capture, so kgen must refuse the
// overload rewrite by name rather than emit an OverloadProg whose LKI is
// silently ignored by the sweep (the kernel would give the land to the
// exiled card's owner, not its last-known controller).
func TestOverloadRememberLKIRefusedByName(t *testing.T) {
	cards := lowerNamed(t, []string{"Winds of Abandon"})
	var c *lCard
	for _, k := range cards {
		if k.Name == "Winds of Abandon" {
			c = k
			break
		}
	}
	if c == nil {
		t.Fatal("precondition: Winds of Abandon not lowered (corpus missing?)")
	}
	var reasons []string
	for _, r := range c.Refusals {
		reasons = append(reasons, r.Reason)
	}
	got := strings.Join(reasons, "; ")
	if !strings.Contains(got, "Overload of a RememberLKI$ Exile") {
		t.Errorf("refusals %q, want the RememberLKI$ overload reason", got)
	}
	if c.OverloadProg != nil {
		for _, in := range c.OverloadProg {
			for _, tok := range strings.Split(in.CS, "|") {
				if tok == "LKI" {
					t.Errorf("OverloadProg still carries LKI: %s", in.String())
				}
			}
		}
	}
}
