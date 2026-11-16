package oracle

import "testing"

// Mana-Charged Dragon's join-forces pay: the attacker chose 10 and holds ten
// sources, one of them a Powerstone token. gorge offers the pay mode; the
// restriction on Powerstone mana bars only spells and abilities (CR 106.6),
// so paying an unless cost may spend it.
func TestMBCManaChargedDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "join_forces_pay_counts_powerstone")
}
