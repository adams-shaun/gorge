package kgen

import (
	"strings"
	"testing"
)

// TestSearchCounterArmDestinations pins gorge's counterDestination split
// (engine/effects/zone_hand.go: battlefield or exile) on the library-search
// rider: battlefield arms the entry; hand/library/graveyard/stack drop it; a
// search to exile is refused because gorge places those counters AFTER the
// move (effects/zone_search.go), which the kernel's single entry arm cannot
// carry. Before this split every search rider was refused, and an
// unconditional drop for non-battlefield destinations silently exiled a card
// bare.
func TestSearchCounterArmDestinations(t *testing.T) {
	lc := &lowerCtx{}
	for _, c := range []struct {
		name string
		dest int
		arm  bool
	}{
		{"battlefield", zBattlefield, true},
		{"hand", zHand, false},
		{"library", zLibrary, false},
		{"graveyard", zGraveyard, false},
		{"stack", zStack, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			arm, clear, err := lc.searchCounterArm(c.dest, "CHARGE", cst(7), cst(1), false)
			if err != nil {
				t.Fatalf("searchCounterArm(%s) error = %v", c.name, err)
			}
			if (arm != nil) != c.arm || (clear != nil) != c.arm {
				t.Fatalf("searchCounterArm(%s) arm=%v clear=%v, want arm=%v", c.name, arm, clear, c.arm)
			}
			if arm != nil && (arm.Op != "SearchCounters" || arm.C != int(15) || arm.B.N != 7) {
				t.Fatalf("battlefield arm = %+v, want SearchCounters CHARGE 7", arm)
			}
		})
	}
	t.Run("exile refuses", func(t *testing.T) {
		if _, _, err := lc.searchCounterArm(zExile, "CHARGE", cst(7), cst(1), false); err == nil ||
			!strings.Contains(err.Error(), "search to exile with WithCountersType$") {
			t.Fatalf("searchCounterArm(exile) error = %v, want the exile refusal", err)
		}
	})
	t.Run("unknown kind", func(t *testing.T) {
		if _, _, err := lc.searchCounterArm(zBattlefield, "NOSUCHKIND", cst(7), cst(1), false); err == nil ||
			!strings.Contains(err.Error(), `WithCountersType$ "NOSUCHKIND"`) {
			t.Fatalf("searchCounterArm(unknown) error = %v, want the kind refusal", err)
		}
	})
}
