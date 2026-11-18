package oracle

import "testing"

// CR 602.5: a has-all-abilities-of ability (Necrotic Ooze) keeps the
// activation restriction of the card it was copied from. Gilt-Blade Prowler's
// "Activate only if you've discarded a card this turn" (CheckSVar$ X, X an SVar
// of the Prowler's face) gates the Ooze's copy on the Ooze controller's own
// discards. gorge read X against the Ooze's face, found no such SVar, and
// failed open, offering the activation on every priority (decision 369 of
// tail-61-c000 pair 56 before the fix).
func TestMBCNecroticOozeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gained_ability_checksvar_gate")
}
