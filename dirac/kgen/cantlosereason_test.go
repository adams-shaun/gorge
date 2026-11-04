package kgen

import (
	"strings"
	"testing"
)

// TestCantLoseReason: "you don't lose the game for having 0 or less life"
// lowers (the kernel stops only that state-based loss); a ValidLoseReason$
// beyond it, or one for every player, refuses with a named reason.
func TestCantLoseReason(t *testing.T) {
	line := "R:Event$ GameLoss | ActiveZones$ Battlefield | ValidPlayer$ You | ValidLoseReason$ LifeReachedZero | Layer$ CantHappen | Description$ Probe."
	if r := kcrimeLower(t, line); len(r) != 0 {
		t.Fatalf("the life-total reason is refused: %v", r)
	}
	for _, c := range []struct{ name, body, want string }{
		{"milled", strings.Replace(line, "LifeReachedZero", "Milled", 1), "GameLoss ValidLoseReason"},
		{"all players", strings.Replace(line, "ValidPlayer$ You", "ValidPlayer$ Player", 1), "GameLoss ValidPlayer"},
	} {
		t.Run(c.name, func(t *testing.T) {
			hit := false
			for _, x := range kcrimeLower(t, c.body) {
				hit = hit || strings.Contains(x.Reason, c.want)
			}
			if !hit {
				t.Fatalf("want a refusal containing %q", c.want)
			}
		})
	}
}
