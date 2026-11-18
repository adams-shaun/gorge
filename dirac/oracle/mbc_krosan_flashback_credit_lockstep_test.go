package oracle

import (
	"testing"
)

// Krosan Reclamation's flashback cast, after the spell resolved into its
// caster's graveyard the same priority series (pair 2 of the tail-53 pool's
// evidence loop, seed 142735): the caster's Inspiring Statuary grants
// improvise to its instant and sorcery spells, so the {1}{G} flashback cost is
// payable through an artifact tap -- a credit. gorge's potential planner
// withholds a credit-carrying potential play whose mode is no
// alternative-cost row ("flashback" is not one): the play is dropped
// ("cost:credit"), and gorge's own priority walk prices the cast from the
// floating pool alone, which an empty pool does not pay. So gorge's canonical
// priority list offers only pass, while the kernel's flashback offer loop --
// priced by its ordinary source census, which the credit route never reaches
// -- offered the cast beside it. The kernel withholds the credit class when
// the floating pool does not already pay the composed cost.
func TestMBCKrosanReclamationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "krosan_flashback_credit")
}
