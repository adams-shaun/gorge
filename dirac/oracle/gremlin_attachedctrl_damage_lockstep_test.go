package oracle

import "testing"

// Gremlin Infestation's end-step trigger deals 2 damage to the enchanted
// artifact's controller (CR 303.4 attachment, CR 120.3a damage to a player).
// The kernel's refCode read that player reference as an object and dealt
// nothing, so its life total stayed 2 above gorge's (tail-65 focus game,
// decision 356). The reference lives in dirac/resolve.go refCode.
func TestGremlinInfestationAttachedControllerDamageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gremlin_infestation_attached_controller_damage")
}
