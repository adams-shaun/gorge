package oracle

import "testing"

// A hybrid pip announced as its blue face is payable with White Lotus
// Hideout's "spend this mana only on a Lesson or Shrine spell" mana (CR
// 601.2b, 601.2g, 106.6): Abandon Attachments is a Lesson, so the cast
// window taps the source's restricted any-colour ability ahead of its plain
// {C}. The dry run priced only the census {C}, called the window stranded and
// reversed the cast; the full walk pays it.
func TestAbandonAttachmentsZoneLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "abandon_attachments_zone", "std-tla", Config{GorgeAutopaySurface: true})
}
