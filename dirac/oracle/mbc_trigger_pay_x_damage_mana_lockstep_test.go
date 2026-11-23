package oracle

import "testing"

// CR 106.1, 605.1a: Nomad Stadium's "{T}: Add {W}. It deals 1 damage to you."
// is a mana ability, so it funds Taj-Nar Swordsmith's "you may pay {X}" when the
// controller has nothing else untapped. Gorge offers X = 1 beside X = 0; the
// kernel once offered only X = 0 (decision 1269 of tail-55 pair 33).
func TestMBCTajNarSwordsmithReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_pay_x_damage_mana_source")
}
