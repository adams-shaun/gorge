package oracle

import "testing"

// TestPinnedFarmGallantCitizen replays the std-spm farm game that diverged with
// Gallant Citizen implicated (SPM-s152, xe:RAND:4:17, turn 17, decision 402,
// Cards[53].Zone). The seat that had floated Interdimensional Web Watch's {W}{W}
// (spendable only on a spell cast from exile) cast Gallant Citizen, a {G/W}{G/W}
// card, from exile by announcing {G} for the first pip and {W} for the second, and
// paid with a Treasure. gorge's colour ask prices the pool AvailableFor holds, the
// restricted batches included, so the floating {W} covered the {W} pip and the
// Treasure answered {G}; dirac's window measured the raw cost against the pool
// beside the restricted mana, answered {W}, found the {G} pip unpayable and
// reversed the cast, leaving the card in exile and the mana floating (CR 106.6,
// 601.2g). The recorded decks are the farm game's, in the sorted decklist order
// that deals its opening at the seed (seat 0 is the tape's p1, which the lower row
// marks swapped); the actions are the explore game up to its turn cap. It is
// written by hand from the farm's tape rather than dealt by the synthetic pool, so
// PINS_RECORD would replace it with a different game.
func TestPinnedFarmGallantCitizen(t *testing.T) {
	t.Parallel()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-spm", rc, 0, onBattlefield("Gallant Citizen"))
}
