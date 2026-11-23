package oracle

import "testing"

// Mesmeric Fiend blinked while its first ETB (exile a card, remember it) is
// pending: the old object's leave trigger returns the old exiled card and
// its own ClearRemembered cleanup must forget only the OLD object's list.
// gorge cleared the re-entered Fiend's live list instead (the new object's
// exiled Hellspark Elemental), so the Fiend's later leave trigger returned
// nothing and the card stayed exiled; the kernel returned it. The recording is
// pair 208 of dr-mbc-abbb (seed 2228289792619997424), 431 decisions.
func TestMBCMesmericFiendLeaveBounceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_mesmeric_fiend_leave_bounce")
}
