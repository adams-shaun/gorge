package oracle

import "testing"

// CR 602.2b with CR 605.1a: Genju of the Fields' {2} is paid, under Rain of
// Filth ("Sacrifice this land: Add {B}."), from tapped lands that the grant
// search sacrifices one at a time. A path that sacrifices the enchanted Plains
// sends Genju to the graveyard with it, so the activation is gone whichever
// way the rest of the path pays; gorge's search ends that line at once
// (potential_script.go leftOwnZone) and goes on to the next land. The kernel's
// search kept expanding the dead line, spent gorge's 160-node budget there
// and dropped the activation gorge offers (dr-mbc-bbba pair 44, decision 863
// of 863).
func TestMBCGenjuOfTheFieldsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "activation_source_sacrificed_in_prefix_ends_the_search")
}
