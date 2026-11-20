package oracle

import "testing"

// CR 603.3b with CR 601.2g-h: Vow of Malice cast off two Eldrazi Spawn
// sacrificed for {C}. Each sacrifice was its own pre-cast activation, so
// Sigil of the New Dawn's may-pay triggers resolved before the cast (both
// decline-only: no {W} left); the kernel left them pending and asked for a
// CR 603.3b order after the cast (dr-mbc-baaa pair 46, decision 481 of 481).
func TestMBCVowOfMaliceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vow_of_malice")
}
