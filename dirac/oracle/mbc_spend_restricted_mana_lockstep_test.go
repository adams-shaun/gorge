package oracle

import "testing"

// CR 106.6, 601.2f: Lorthos, the Tidemaker and Sauron ({5}{U}{U}{U} / {5}{U}{B}{R}, both nine mana)
// in a hand whose untapped mana is six basics, a Desolate Mire and a Powerstone token (its {C}
// can't be spent on a nonartifact spell), under Mystic Monstrosity. gorge offered both casts;
// dirac offered neither (decision 508 of the recording).
func TestMBCLorthosTheTidemakerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spend_restricted_mana_lorthos_affordability")
}
