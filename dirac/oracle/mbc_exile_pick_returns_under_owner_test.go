package oracle

import "testing"

// Sengir Nosferatu's Bat token is "{1}{B}, Sacrifice this creature: Return an
// exiled card named Sengir Nosferatu to the battlefield under its owner's
// control." A stolen Bat (its controller is not the exiled card's owner) still
// returns the Nosferatu under its OWNER: an object a hidden pick returns to the
// battlefield enters under the moved object's owner, the resolving source's
// controller only with a GainControl$ (CR 610.3c's family; gorge
// settleChangeZoneMoveAs passes o.Owner). The kernel entered it under the
// resolving controller (decision 350; tail-60 pair 178, seed 3, the game the
// evidence loop filed under Sengir Nosferatu).
func TestMBCSengirNosferatuReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_pick_returns_under_owner")
}
