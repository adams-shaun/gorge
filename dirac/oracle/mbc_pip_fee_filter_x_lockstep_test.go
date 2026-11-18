package oracle

import "testing"

// CR 602.2b with CR 107.4: Apprentice Wizard's "{U}, {T}: Add {C}{C}{C}" costs
// a {U}, which only blue mana pays. With only {G} and {W} floating and every
// Island tapped, the window cannot activate it, so Tyvar's Stand ({X}{G}) is
// offered X = 0..1. The kernel paid the fee as a count from any colour and
// offered X up to 3 (dr-mbc-baab pair 24, decision 414).
func TestMBCTyvarSStandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pip_fee_filter_x_ask_needs_its_colour")
}
