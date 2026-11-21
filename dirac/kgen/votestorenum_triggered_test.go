package kgen

import (
	"strings"
	"testing"
)

// TestVoteStoreNumTriggeredHasNoPlacementMode pins that a triggered
// StoreVoteNum$ Vote lowers with no OpCharm block and no modal ability: gorge
// poses no CR 603.3c placement mode for a triggered Vote (ceace7cc00), so the
// kernel asks the ballot only, at resolution.
func TestVoteStoreNumTriggeredHasNoPlacementMode(t *testing.T) {
	triggered := []string{"Lieutenants of the Guard", "Messenger Jays", "Orchard Elemental", "Emissary Green"}
	lir, ref := lowerCarriers(t, triggered)
	for _, n := range triggered {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
		sec := section(lir, n)
		if !strings.Contains(sec, "Vote C=") {
			t.Errorf("%s: no Vote op lowered (precondition):\n%s", n, sec)
		}
		if strings.Contains(sec, "Charm") {
			t.Errorf("%s: a triggered Vote carries a placement Charm:\n%s", n, sec)
		}
		c := lowerCorpusCard(t, n)
		for _, ab := range c.Abilities {
			if ab.Modes != 0 {
				t.Errorf("%s: ability carries %d modes, want none", n, ab.Modes)
			}
		}
	}
}
