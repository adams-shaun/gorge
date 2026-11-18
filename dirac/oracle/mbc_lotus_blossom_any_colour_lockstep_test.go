package oracle

import "testing"

// CR 605.1a, 106.4: Lotus Blossom ("{T}, Sacrifice: Add X mana of any one
// color, X the petal counters") is the only green source on p1's board, so
// gorge offers Contortionist Troupe (X=0, {G}) off its one petal counter
// (tail-74-c000, seed 370164, decision 493). The kernel's counter-scaled
// sacrifice gap once read one fixed colour and offered nothing; it now asks
// the colour its activation needs (mana_sacctr.go ctrAnyColour, be4db3171d),
// so the recorded game replays clean and the card is no longer refused.
func TestMBCContortionistTroupeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lotus_blossom_any_colour_sac_mana")
}
