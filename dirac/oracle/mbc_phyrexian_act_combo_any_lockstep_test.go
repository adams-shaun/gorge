package oracle

import "testing"

// CR 107.4f, 601.2g: Zopandrel's {G/P}{G/P}, sacrifice two other creatures
// activation announces one pip as {G} and the other as two life; the only
// untapped mana source is Axebane Guardian ({T}: Add X mana in any combination
// of colors), which gorge's payment window taps for the {G} while the
// activation is on the stack (decision 362 of dr-mbc-abbb pair 2).
func TestMBCAxebaneGuardianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_act_pip_funded_by_combo_any")
}
