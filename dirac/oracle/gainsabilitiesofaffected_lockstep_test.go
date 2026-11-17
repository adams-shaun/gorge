package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kgainsofAffectedDecks: Hazel's Brewmaster gives every Food its controller
// controls the activated abilities of the creature cards it exiled
// (Affected$ Food.YouCtrl, GainsAbilitiesOf$ Creature.ExiledWithSource,
// GainsAbilitiesOfZones$ Exile). Brewmaster and Brandywine Farmer make the
// Food; Prodigal Sorcerer, Jeska and Prava carry non-mana activated abilities
// (a gained mana ability stalls the walk, StallGainsOf) and die in combat or
// to their own pings, so gorge offers a Food the exiled creature's ability.
func kgainsofAffectedDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Hazel's Brewmaster", Count: 4},
		{Name: "Brandywine Farmer", Count: 4},
		{Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Jeska, Warrior Adept", Count: 4},
		{Name: "Prava of the Steel Legion", Count: 4},
		{Name: "Swamp", Count: 6},
		{Name: "Forest", Count: 2},
		{Name: "Island", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Plains", Count: 4},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// TestGainsAbilitiesOfAffectedLockstep: the recipient walk of
// dirac/static_gainsof.go agrees with gorge on every game, and gorge offered
// at least one gained activation (GainedSource != 0) in them.
func TestGainsAbilitiesOfAffectedLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "gainsofaffected", kgainsofAffectedDecks(), 40, true, nil, nil, kgainsofPolicy(st))
	if (*st)["gained"] == 0 {
		t.Fatal("gorge never offered a gained activation in the games; the lockstep proves nothing")
	}
	t.Logf("priority decisions offering a gained activation: %d", (*st)["gained"])
}
