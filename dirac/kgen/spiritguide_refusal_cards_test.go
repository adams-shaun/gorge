package kgen

import (
	"strings"
	"testing"
)

// TestSpiritGuideRefusal pins the scoping decision for the Spirit Guides
// (branch C): both stay refused, each by exactly one named reason about the
// missing hand source in the kernel's mana window, and neither reaches the
// kernel's card table.
func TestSpiritGuideRefusal(t *testing.T) {
	const want = "the kernel has no hand source in its mana window"
	names := []string{"Simian Spirit Guide", "Elvish Spirit Guide"}
	cs := lowerNamed(t, names)
	seen := 0
	for _, c := range cs {
		if c.Name != names[0] && c.Name != names[1] {
			continue
		}
		seen++
		var named []string
		for _, r := range lowerRefusals(c) {
			if strings.Contains(r.Reason, want) {
				named = append(named, r.Reason)
			}
		}
		t.Logf("%s refusals: %v", c.Name, lowerRefusals(c))
		if len(named) != 1 {
			t.Errorf("%s: %d refusals name the hand source (%v), want exactly 1; all: %v", c.Name, len(named), named, c.Refusals)
		}
	}
	if seen != 2 {
		t.Fatalf("lowered %d of the two Spirit Guides", seen)
	}
}
