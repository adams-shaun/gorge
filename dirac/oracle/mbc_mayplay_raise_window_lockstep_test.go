package oracle

import "testing"

// CR 601.2g with CR 118.9: Urza, Lord High Artificer's {5} ability exiled
// Lay Claim and granted its may-play cast, and the opponent's Kasmina,
// Enigmatic Mentor taxes the cast {2} whenever it targets a creature or
// planeswalker she controls (CR 601.2c announces targets before the total
// cost is read at 601.2f). The pool is empty, so the taxed {2} is payable
// only through the payment window: Island and Urza's "tap an untapped
// artifact: Add {U}" make exactly {U} twice. gorge's affordableTarget
// probe prices the raise from that window and keeps the taxed targets on
// the menu; the kernel's dropTargetsOwing skipped the window fallback for a
// free (may-play) cast and dropped them (decision 891 of 891; dr-mbc-baaa
// pair 52, seed 9246291670666261013).
func TestMBCCorpseConnoisseurReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_raise_window")
}
