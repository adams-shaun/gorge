package oracle

import "testing"

// TestMBCVaultSkirgePipFilterOrderLockstep replays the recorded tail-01 game (pair 10)
// where gorge offered choose:pay(-,-,2), paying Vault Skirge's Phyrexian {B/P}
// with 2 life, at decision 285 and dirac offered only the 7-mana payment. The
// kernel's filter-land search ordered the Phyrexian pip behind the filter chain
// (fixed by the filter-order walk in dirac/mana_filterorder.go).
func TestMBCVaultSkirgePipFilterOrderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "vault_skirge_phyrexian_pip_filter_order")
}
