package oracle

import "testing"

// Condition$ AttackedPlayerWithMostLife on Scourge of the Throne's attack
// trigger: gorge fires it only when the attacked player has the most life or
// is tied (trigmatch.PlayerHasMostLife, the Dethrone gate). dirac once read the
// condition as ignored and queued the untap trigger on every first attack, so
// the trigger_order ask offered a third order the gorge side never posed
// (decision 744 of tail-13-c000 pair 4).
func TestMBCScourgeOfTheThroneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attack_most_life_condition")
}
