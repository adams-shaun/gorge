package oracle

import "testing"

// CR 508.3a and CR 603.2 with CR 508.1: an AttackersDeclared trigger whose
// ValidAttackers$ is Card.Self fires only when the trigger's own source is one
// of the declared attackers. Merry, Esquire of Rohan ("Whenever you attack
// with Merry and another legendary creature, draw a card") watched a
// declaration of Iraxxa, Empress of Mars and Thriving Grubs and the kernel
// offered its trigger anyway: the trigger builder lifted the sole Card.Self
// clause to the TfSelf flag, which the AttackersDeclared path never reads, so
// the empty filter counted every attacker.
func TestMBCMerryEsquireOfRohanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attackers_valid_self_must_attack")
}
