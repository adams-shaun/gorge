package oracle

import "testing"

// CR 605.3a with Urza, Lord High Artificer ("Tap an untapped artifact you
// control: Add {U}"): a player paying an activation may activate that mana
// ability once per untapped artifact, so Shield of the Ages' {2} is affordable
// beside Urza and untapped artifacts. The recorded game (tail-64 pair 157)
// parts at an offer of Shield of the Ages' activation at decision 716.
func TestMBCShieldOfTheAgesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "shield_of_the_ages_urza_tap_artifact_afford")
}
