package oracle

import "testing"

// CR 613.1f with CR 605.1a and CR 601.2g: Hazel's Brewmaster's static gives
// each Food it created the activated abilities of the creature cards exiled
// with it (GainsAbilitiesOfZones$ Exile), so the Food on the battlefield has
// the exiled Blood Pet's "Sacrifice: Add {B}". That granted mana ability is
// a potential-action mana source (gorge's PotentialMana fixpoint admits a
// paid-activation source), so its controller can pay {4}{W} for Blessed
// Light from two Plains, a Swamp and the Food's black — gorge offers the
// cast. The kernel's planner read GainsOf statics' mana grants from
// battlefield permanents only (gainedIdxs skipped the zone arms), saw four
// mana, and withheld it.
func TestMBCBlessedLightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_gainsof_mana")
}
