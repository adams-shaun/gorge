package oracle

import "testing"

// CR 400.7, 701.19a: Silvos, Rogue Elemental's "{G}: Regenerate" is on the
// stack when Restoration Angel blinks Silvos. The returned Silvos is a new
// object, so the resolving ability grants it no regeneration shield. gorge
// shielded the new permanent anyway (tail-55 focus game, decision 934).
func TestMBCSilvosShieldCounterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "regenerate_after_source_blinked_gives_no_shield")
}
