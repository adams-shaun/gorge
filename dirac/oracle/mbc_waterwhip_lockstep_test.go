package oracle

import "testing"

// CR 601.2a, 601.2f: Water Whip's "As an additional cost to cast this spell,
// waterbend {5}" is owed by whoever casts it, and its Activator$ You reads the
// caster, not the card's owner. Siphon Insight exiled the opponent's Water Whip
// face down and lets its controller cast it with any colour of mana; with
// untapped Swamps, two artifact creatures and an artifact, the {U}{U} plus
// waterbend {5} is payable only through the planner-declined waterbend shape,
// so gorge's autopay surface offers no cast at all. The kernel read the
// waterbend as the owner's alone, priced the cast at {U}{U} and offered it
// (tail-07 pair 2, decision 777).
func TestMBCWaterWhipReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "non_owner_cast_owes_own_waterbend")
}
