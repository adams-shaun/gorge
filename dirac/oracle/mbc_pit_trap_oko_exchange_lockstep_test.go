package oracle

import "testing"

// CR 603.7 with RememberObjects$ RememberedLKI: a delayed trigger whose
// remembered set is the set a ChangeZoneAll with RememberChanged$ wrote —
// Planar Guide's "Exile all creatures. At the beginning of the next end step,
// return those cards to the battlefield under their owners' control" —
// registers the WHOLE set, and its body returns all of it. The kernel lowered
// the record to its one-object delayed program, so it returned only the first
// exiled creature and left the rest in exile (Ghostway the same shape).
// The recording is the tail-53 evidence pool's pair-614 game (Oko, Thief of
// Crowns' -5 exchange of Linessa, Zephyr Mage and Merchant of Truth, Pit Trap
// and Quicksilver Amulet in play), continued past its recorded first
// divergence: the kernel returned only Shanna, Sisay's Legacy and left
// Merchant of Truth and Syndicate Messenger exiled.
func TestMBCPitTrapOkoExchangeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_return_of_changezoneall_set")
}
