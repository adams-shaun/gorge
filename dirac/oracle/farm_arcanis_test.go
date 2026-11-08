package oracle

import "testing"

// The std-fdn farm game that diverged with Arcanis the Omnipotent implicated
// (xe:RAND:1:9, run FDN-s443, turn 28) is pinned from its tape: the tape's seats
// are swapped relative to the game (its lower row has swap=true), so the game's
// seat 0 is the tape's p1 deck (Pyromancer's Goggles) and its seat 1 the tape's
// p0 deck (New Horizons). At decision 682 dirac left out an activation whose
// payment the grant/deferred script search must count; 075bac118 fixed that
// prefix search (CR 605.1a), and this pin must fail again if it regresses.
func TestPinnedFarmArcanistheOmnipotent(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-fdn", Config{GorgeAutopaySurface: true}, onBattlefield("Arcanis the Omnipotent"))
}
