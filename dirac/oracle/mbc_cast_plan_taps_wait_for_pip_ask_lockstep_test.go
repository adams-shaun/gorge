package oracle

import "testing"

// CR 601.2b, 601.2g: a cast announces its Phyrexian pips before any mana is
// paid. Moltensteel Dragon ({4}{R/P}{R/P}) cast with a planned payment through
// Thornglint Bridge and Selesnya Signet: the kernel tapped them before the pip
// ask, gorge after it (decision 400, tail-37 focus game).
func TestMBCMoltensteelDragonCastPaymentShapeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_plan_taps_wait_for_pip_ask")
}
