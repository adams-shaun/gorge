package oracle

import "testing"

// A Blight cost (Sting-Slinger's "{1}{R}, {T}, Blight 1") answered with Fiend
// Hunter (decision 576 of dr-mbc-babb pair 30): gorge and dirac must agree on
// where the -1/-1 counter lands.
func TestMBCFiendHunterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blight_cost_on_fiend_hunter")
}
