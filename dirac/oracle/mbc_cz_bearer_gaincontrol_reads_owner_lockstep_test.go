package oracle

import (
	"testing"
)

// Nomad Mythmaker's "{W}, {T}: Put target Aura card from a graveyard onto the
// battlefield under your control attached to a creature you control" returned
// the opponent's Dead Man's Chest ("Enchant creature an opponent controls")
// with no creature the Chest can legally enchant left on its entering
// controller's side: the Chest stays in its zone (CR 303.4g) and the ability
// resolves whole. The kernel instead read the Chest's enchant restriction
// under its owner and asked the bearer election gorge never posed (pair 44,
// synth tail-06, surface full, mulligans 0).
func TestMBCGainControlBearerAskReadsOwnerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cz_bearer_gaincontrol_reads_owner")
}
