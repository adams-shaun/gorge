package oracle

import "testing"

// CR 704.5j's legend rule groups by NAME, and a token's name is its script's
// Name: line (CR 111.4): the pre-existing w_1_1_soldier "Soldier Token" and
// Mass Production's four c_1_1_a_soldier "Soldier Token" artifact tokens are
// one duplicate set under Leyline of Singularity's layer-4 Legendary grant,
// so the keep choice offers all five (gorge e.Name; the kernel's CardID
// grouping offered only the four Mass Production tokens, whose script stem
// differs). Recorded from the tail-59 evidence loop (pair 150, seed
// 6178433022764609066) at the legend ask, decision 701.
func TestMBCMassProductionReprosLockstep(t *testing.T) {
	stdReproNamed(t, "legend_token_name")
}
