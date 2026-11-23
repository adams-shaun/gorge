package oracle

import "testing"

// CR 106.1 and CR 602.2: General Tazri's {W}{U}{B}{R}{G} ability is payable
// from five Prism tokens, each sacrificed to Diamond Kaleidoscope for one mana
// of any color, tapped or not (a sacrifice cost doesn't need an untapped
// source). Gorge offers the activation at decision 814 (tail-12-c000 pair 47);
// dirac offered only pass.
func TestMBCGeneralTazriReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacrifice_mana_sources_pay_five_colors")
}
