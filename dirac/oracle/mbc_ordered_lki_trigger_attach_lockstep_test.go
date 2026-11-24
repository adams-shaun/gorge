package oracle

import "testing"

// Sword of the Meek's "return it, then attach it to that creature" with a
// graveyard Sword and thirteen 1/1 Faerie Dragon tokens entering at once
// (Ancient Gold Dragon's d20): the thirteen triggers are ordered by the
// controller, and the one answered "yes" must attach the Sword to its own
// token (tail-23-c000 pair 3, decision 707).
func TestMBCSwordOfTheMeekReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ordered_lki_trigger_attach")
}
