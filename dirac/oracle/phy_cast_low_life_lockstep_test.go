package oracle

import "testing"

// CR 107.4f, CR 119.4: a Phyrexian pip may be paid with two life only when
// the payer's life total is at least 2, so at 1 life every pip of a cast must
// be paid with its colour. Moltensteel Dragon's {4}{R/P}{R/P} therefore needs
// six mana (four generic and two red) from four sources, and the cast is
// illegal; the kernel's cast offer gate priced only the pip-free mana half and
// offered it (phyact pool, seed 510, decision 254).
func TestPhyCastLowLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phy_cast_low_life")
}
