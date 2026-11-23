package oracle

import "testing"

// CR 602.5b: Sliver Hive's "{5},{T}: create a Sliver. Activate only if you
// control a Sliver" is payable at decision 377 of pool tail-73 (pair 42, seed
// 537512) by Forest, Island, Base Camp and Phyrexian Tower's "{T}, Sacrifice a
// creature: Add {B}{B}", and the only Sliver p1 controls is the changeling
// Chameleon Colossus beside Jeering Homunculus. gorge's prefix search tried
// the Colossus as the Tower's sacrifice first, priced the play from the BB it
// floats, and ended the search there: the line left no Sliver, so the play was
// no longer offered and the whole search came back with nothing. The search
// now treats a prefix that breaks the activation's IsPresent gate as no end,
// so the Homunculus branch prices the play and Hive is offered, as the
// kernel's menu already did (options options, lead gorge, implicates Sliver
// Hive).
func TestMBCSliverHiveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "script_prefix_sac_breaks_present_gate")
}
