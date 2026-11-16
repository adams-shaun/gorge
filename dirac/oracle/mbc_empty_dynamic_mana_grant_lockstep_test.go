package oracle

import "testing"

// CR 605.1a, 106.1, 903.4: a land whose own mana ability makes no mana now
// (Hidden Hideout's "one mana of any color in your commander's color
// identity", which is nothing without a commander) still taps for the mana
// an Aura grants it. Weirding Wood on p0's Hidden Hideout grants "{T}: Add
// two mana of any one color"; gorge priced p0's {7} Hexplate Golem and
// {6}{R} Minotaur Aggressor through that grant, and the kernel counted the
// Hideout as an own source making 0 and so never read the grant. The
// recording is the first divergence of tail-20 pair 9 (focus Volatile Claws,
// six per deck), decision 553, gorge leading, on the full surface.
func TestMBCEmptyDynamicManaSourceGrantLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "empty_dynamic_mana_source_counts_granted_ability")
}
