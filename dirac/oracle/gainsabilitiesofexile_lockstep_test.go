package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kgainsofExileDecks: Dark Impostor ({4}{B}{B}: exile target creature) has
// every activated ability of the creature cards it exiled
// (GainsAbilitiesOf$ Creature.ExiledWithSource, GainsAbilitiesOfZones$ Exile).
// The other creatures carry non-mana activated abilities (a gained mana
// ability stalls the walk, StallGainsOf), so once Impostor exiles one, gorge
// offers Impostor its ability from exile.
func kgainsofExileDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Dark Impostor", Count: 4},
		{Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Jeska, Warrior Adept", Count: 4},
		{Name: "Prava of the Steel Legion", Count: 4},
		{Name: "Swamp", Count: 12},
		{Name: "Island", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Plains", Count: 4},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// TestGainsAbilitiesOfExileLockstep: the exile walk of dirac/static_gainsof.go
// agrees with gorge on every game, and gorge offered at least one gained
// activation (GainedSource != 0) in them.
func TestGainsAbilitiesOfExileLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "gainsofexile", kgainsofExileDecks(), 40, true, nil, nil, kgainsofPolicy(st))
	if (*st)["gained"] == 0 {
		t.Fatal("gorge never offered a gained activation in the games; the lockstep proves nothing")
	}
	t.Logf("priority decisions offering a gained activation: %d", (*st)["gained"])
}

// TestGainsAbilitiesOfExileMutation: with the kernel's gained-ability walk
// off (dirac.DebugNoGainsOf) the exile decks diverge: the divergence is the
// exile walk's, as these decks gain nothing from the battlefield. Not
// parallel: the flag is global.
func TestGainsAbilitiesOfExileMutation(t *testing.T) {
	old := dirac.DebugNoGainsOf
	dirac.DebugNoGainsOf = true
	defer func() { dirac.DebugNoGainsOf = old }()
	st := new(map[string]int)
	if n := lockstepRunPolicy(t, "gainsofexile", kgainsofExileDecks(), 40, false, nil, nil, kgainsofPolicy(st)); n == 0 {
		t.Fatal("no seed diverged with the gained-ability walk off")
	}
}
