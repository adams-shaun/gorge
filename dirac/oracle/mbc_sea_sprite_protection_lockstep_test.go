package oracle

import "testing"

// CR 702.16: a creature that already prints "protection from red" still gains
// Akroma's Memorial's "protection from black and from red" when the Memorial
// enters. The projection drops a printed protection spelling from gorge's
// keyword list (the kernel carries it as CardDef.ProtColors), so the kernel's
// projected bit for the same printed colour must be dropped too: Sea Sprite
// shows black only, not black and red (decision 601, tail-29 focus game).
func TestMBCSeaSpriteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sea_sprite_protection")
}
