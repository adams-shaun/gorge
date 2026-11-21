package kgen

import "testing"

// TestTriggeredVoteStoreNumHasNoModes pins that a triggered StoreVoteNum$
// fixed-list Vote lowers with no modes and no Charm op: gorge poses no
// CR 603.3c placement mode for a Vote (askTriggerModes returns false for
// it), so a kernel that asked one diverged at the cast of the carrier.
func TestTriggeredVoteStoreNumHasNoModes(t *testing.T) {
	for _, n := range []string{"Messenger Jays", "Lieutenants of the Guard", "Orchard Elemental", "Emissary Green"} {
		c := lowerCorpusCard(t, n)
		votes := 0
		for _, ab := range c.Abilities {
			if ab.Modes != 0 {
				t.Errorf("%s: ability carries %d modes", n, ab.Modes)
			}
			for _, in := range ab.Prog {
				switch in.Op {
				case "Charm":
					t.Errorf("%s: Charm op lowered", n)
				case "Vote":
					votes++
				}
			}
		}
		if votes != 1 {
			t.Errorf("%s: %d Vote ops, want 1", n, votes)
		}
	}
}
