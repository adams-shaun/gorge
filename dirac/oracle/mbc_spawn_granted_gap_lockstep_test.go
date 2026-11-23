package oracle

import "testing"

// CR 605.3b, 601.2h: Murderous Rider's Swift End ({1}{B}{B}) is payable only
// through Cave of Temptation's {1},{T} any-colour filter. The board also holds
// two tokens under Jaheira's granted "{T}: Add {G}": a Wizard and an Eldrazi
// Spawn whose own "Sacrifice this creature: Add {C}" sits beside the grant.
// gorge's prefix search walks the Spawn as a gap with a "choose a mana
// ability" ask and as a sacrifice funder once it is tapped, so it spends its
// 160-node budget (209 nodes needed) and leaves Swift End unoffered; the
// kernel's search never branched on the Spawn, found the script in 35 nodes
// and offered the cast (decision 488 of tail-76 pair 177, autopay).
func TestMBCMurderousRiderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spawn_granted_tap_gap_prefix_search")
}
