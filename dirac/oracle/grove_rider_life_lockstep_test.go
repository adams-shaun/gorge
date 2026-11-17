package oracle

import "testing"

// TestGroveRiderLifeReprosLockstep pins the payment where Grove of the
// Burnwillows' plain "{T}: Add {C}" ability and its "{T}: Add {R} or {G}.
// Each opponent gains 1 life." rider are both usable: the lockstep script must
// put the same ability on the stack, or the life total parts (CR 605.3b: the
// rider resolves with the ability's mana, at activation).
func TestGroveRiderLifeReprosLockstep(t *testing.T) {
	stdReproNamed(t, "grove_rider_life")
}
