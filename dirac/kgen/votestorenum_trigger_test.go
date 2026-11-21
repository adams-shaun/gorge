package kgen

import (
	"strings"
	"testing"
)

// TestVoteStoreNumTriggerHasNoPlacementMode: gorge no longer poses a CR 603.3c
// placement mode for a triggered Vote (askTriggerModes), so a StoreVoteNum$
// trigger lowers as one Vote with its branches and no Charm wrapper; a Charm
// would make the kernel ask a mode gorge never offers.
func TestVoteStoreNumTriggerHasNoPlacementMode(t *testing.T) {
	names := []string{"Lieutenants of the Guard", "Orchard Elemental", "Messenger Jays", "Emissary Green"}
	lir, _ := lowerCarriers(t, names)
	for _, n := range names {
		sec := section(lir, n)
		if strings.Count(sec, " Vote C=") != 1 {
			t.Errorf("%s: want exactly one Vote op (precondition: the carrier lowers):\n%s", n, sec)
		}
		if strings.Contains(sec, "Charm") || !strings.Contains(sec, "modes=0") {
			t.Errorf("%s: a triggered StoreVoteNum$ vote carries a placement mode:\n%s", n, sec)
		}
	}
}
