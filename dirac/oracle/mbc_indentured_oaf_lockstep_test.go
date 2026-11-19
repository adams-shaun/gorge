package oracle

import "testing"

// CR 608.2b with CR 701.15a: Goblin Kaboomist's Land Mine token is sacrificed
// as the cost of its "{R}, Sacrifice this artifact: It deals 2 damage to target
// attacking creature without flying" ability. The source is gone when the
// ability resolves, but the damage is still dealt; the kernel dropped it while
// gorge marked 2 on Indentured Oaf (decision 641 of 641).
func TestMBCIndenturedOafReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "indentured_oaf")
}
