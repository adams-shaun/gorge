package oracle

import "testing"

// dr-mbc-bbbb pair 124 (land 7bfdc06e58): a Strangleroot Geist attacks past
// Caltrops; the kernel marks its combat damage and gorge does not (or vice
// versa), so the next projection parts at Cards[37].Damage.
func TestMBCStranglerootGeistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "strangleroot_damage_projection")
}
