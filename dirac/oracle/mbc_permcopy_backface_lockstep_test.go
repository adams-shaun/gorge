package oracle

import "testing"

// CR 707.10f, 707.10g: Tomb of Horrors Adventurer's trigger copies a cast
// Titania, Voice of Gaea (a modal double-faced card cast front face up); the
// copy becomes a token as it resolves. The kernel once stalled `copyface` on
// the copy instead of playing it (tail-76-c000 pair 42, decision 774).
func TestMBCTail76ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permcopy_backface_titania")
}
