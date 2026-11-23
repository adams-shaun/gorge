package oracle

import "testing"

// CR 119.4 and CR 603.2: with Manabarbs out, every land p1 taps for mana deals
// p1 one damage, so at 4 life Cleaving Sliver's {3}{R} (four taps from a
// five-land board) would kill its caster. gorge's payment planner cuts any
// plan whose summed damage reaches the payer's life, so it does not list the
// cast; dirac's per-unit census capped each colour on its own and listed it
// (decision 259 of 259, tail-34-c000 pair 125).
func TestMBCCleavingSliverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "manabarbs_tap_pain_joint_lethal_guard")
}
