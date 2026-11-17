package oracle

import "testing"

// CR 605.3b with CR 601.2b: a filter's fee in the payment window is paid out
// of the mana made so far, and a unit the fee spent pays no pip of its colour
// after. Vault Skirge's {1}{B/P} beside Sunscorched Divide ("{1}, {T}: Add
// {R}{W}") and a lone Swamp: the only B funds the pip or the Divide's fee,
// never both, so the {B/P} pip's colour face is not payable and the ask offers
// the two-life face alone. The kernel's pip ask read the fee off the avail's
// total alone and still counted the spent B, offering a colour face no legal
// assignment can pay (decision 54 of tail-01 focus pair 199, gorge leading,
// on the autopay surface).
func TestMBCVaultSkirgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vault_skirge_pip_filter_fee")
}
