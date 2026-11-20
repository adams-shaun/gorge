package kgen

import "testing"

// TestVoteStoreNumTriggeredHasNoPlacementMode pins that a triggered
// StoreVoteNum$ fixed-list Vote lowers to one Vote op and no modal ability:
// gorge poses no CR 603.3c placement mode for a triggered Vote
// (askTriggerModes), so a Charm wrapper would make the kernel ask a mode gorge
// never does.
func TestVoteStoreNumTriggeredHasNoPlacementMode(t *testing.T) {
	for _, n := range []string{"Lieutenants of the Guard", "Orchard Elemental", "Emissary Green", "Messenger Jays"} {
		c := lowerCorpusCard(t, n)
		votes := 0
		for _, ab := range c.Abilities {
			if ab.Kind != akTriggered {
				continue
			}
			if ab.Modes != 0 {
				t.Errorf("%s: triggered ability has %d modes, want none", n, ab.Modes)
			}
			for _, in := range ab.Prog {
				switch in.Op {
				case "Charm":
					t.Errorf("%s: Charm op in a triggered Vote", n)
				case "Vote":
					votes++
				}
			}
		}
		if votes != 1 {
			t.Errorf("%s: %d Vote ops in triggered abilities, want 1", n, votes)
		}
	}
}
