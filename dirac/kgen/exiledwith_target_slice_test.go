package kgen

import (
	"strings"
	"testing"
)

// TestExiledWithTargetSlice: a ChangeZone Origin$ Exile target filter that
// reads ExiledWithSource lowers (Nyla's end-step trigger); the kernel reads the
// link against the entry's source at target time. Bronzebeak Foragers and
// Gelatinous Cube read it too: their target filter no longer refuses, and the
// movers they sit behind (an exile->graveyard target move, a duration exile
// read against the link) are the refusals still named.
func TestExiledWithTargetSlice(t *testing.T) {
	names := []string{"Nyla, Shirshu Sleuth", "Bronzebeak Foragers", "Gelatinous Cube"}
	cs := lowerNamed(t, names)
	if len(cs) != len(names) {
		t.Fatalf("lowered %d of %d cards", len(cs), len(names))
	}
	for _, c := range cs {
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "ExiledWithSource outside") {
				t.Errorf("%s: target filter refused: %s", c.Name, r.Reason)
			}
			if c.Name == "Nyla, Shirshu Sleuth" {
				t.Errorf("%s: refused: %s", c.Name, r.Reason)
			}
		}
		if c.Name == "Nyla, Shirshu Sleuth" && !c.ExiledWith {
			t.Errorf("%s: lowered but not marked as an ExiledWith reader", c.Name)
		}
	}
}
