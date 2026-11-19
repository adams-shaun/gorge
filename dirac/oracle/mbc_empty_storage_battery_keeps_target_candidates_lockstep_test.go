package oracle

import "testing"

// CR 601.2c, 601.2h: No One Left Behind's {3} reduction reads its target
// (ValidTarget$ Creature.cmcLE3), so only a creature with mana value 3 or
// less makes the {4}{B} cast cost {1}{B}. The target menu is announced
// before the total cost is determined and paid, and the caster's empty
// storage battery (Sand Silos, "{T}, Remove any number of storage counters:
// Add {U} for each") still counts as an untapped mana source whose window
// gorge's census cannot price. The pool-short target menu therefore keeps
// every legal graveyard creature card -- Bone Miser and Clockwork Condor
// included -- even though only Firebird, Blazing Ranger makes the cast
// affordable; the cast reverses at payment (CR 601.2h, 733.1) when the
// chosen target leaves it unpayable. The kernel filtered the menu to the
// reduction-matching target (decision 727 of 727; tail-23 focus game).
func TestMBCBoneMiserReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "empty_storage_battery_keeps_target_candidates")
}
