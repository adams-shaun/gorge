package oracle

import (
	"testing"
)

// A face cast's lowering witness sacrifices a mana source (a Treasure), and
// the sacrifice triggers an optional-pay election (Sorcerer's Broom's
// "Whenever you sacrifice another permanent, you may pay {3}"). gorge's
// lowering answers the election by its first option -- pay -- which drains
// the floated mana off the plan's prediction (payexec's pool_mismatch), so
// the play is one gorge's surface never offers ("lower dead end"). The
// kernel's face-cast offer mirror (face_castface.go faceLowerDiscardsCast)
// priced only the witness's life-cost discard class, so it offered Boulderbranch
// Golem's prototyped cast {3}{G} gorge had dropped (tail-70 pair 63, seed
// 8052064013195788656, decision 445).
func TestMBCBoulderbranchGolemReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "face_witness_sac_paydrain")
}
