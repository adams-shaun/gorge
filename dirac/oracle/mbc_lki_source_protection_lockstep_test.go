package oracle

import "testing"

// CR 608.2b with CR 113.7a and CR 702.16b: Artisan of Forms, a copy of Festering Goblin,
// dies and its "target creature gets -1/-1" trigger is a black source by last known
// information. Rakdos Pit Dragon wearing Sword of Feast and Famine (protection from
// black) is not a legal target; gorge read the blue card in the graveyard and offered it
// (decision 456, tail-06-c000 pair 69).
func TestMBCRakdosPitDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lki_source_protection_target")
}
