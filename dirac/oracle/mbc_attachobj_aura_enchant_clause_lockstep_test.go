package oracle

import "testing"

// Codsworth, Handy Helper's "Attach target Aura or Equipment you control to
// target creature you control" with Coral Net (Enchant green or white
// creature) and a creature that is neither: CR 701.3b, the Aura does not move,
// so it stays on Codsworth. The kernel's OpAttachObj fastened it to the
// illegal host and the state-based action sent it to the graveyard (tail-68
// pair 29, decision 188).
func TestMBCAttachObjAuraEnchantClauseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attachobj_aura_enchant_clause_rejects_destination")
}
