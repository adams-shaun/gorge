package oracle

import "testing"

// CR 701.17a and Forge's DigUntilEffect: a reveal-until that puts the
// revealed cards into their graveyard (no FoundDestination$: Mirko Vosk, Mind
// Drinker's "reveals cards from the top of their library until they reveal
// four land cards, then puts those cards into their graveyard") moves the
// pile in reveal order, the found lands at their own scan positions. Dirac
// appended the found cards after the other revealed cards, so the lands sat
// on top of the graveyard batch (decision 414 of a synth tail-47 Mirko focus
// game; the lockstep class was projection Cards[N].Pos across the pile).
func TestDigUntilFoundRevealedScanOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "diguntil_found_revealed_scan_order")
}
