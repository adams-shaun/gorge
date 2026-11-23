package oracle

import "testing"

// Reclaiming Vines ({2}{G}{G} sorcery, "Destroy target artifact, enchantment,
// or land") is offered by one engine at a priority decision the other withholds
// (tail-15 pair 0, decision 948, surface full). Recorded on a custom pool:
// tail-15's membership moves.
func TestMBCReclaimingVinesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reclaiming_vines_offer")
}
