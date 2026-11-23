package oracle

import (
	"testing"
)

// TestMBCRemPersistBlinkLiveReadReprosLockstep replays the recorded
// dr-mbc-abbb game (focus pool, decks focus_00231_WBR vs focus_00049_WBR,
// seed 17403837841587922922, pair 536): Mesmeric Fiend's first enters trigger
// was still on the stack when a Cloudshift blinked the Fiend, so its leaves
// trigger's fire-time capture was empty. The re-entry's enters trigger then
// resolved first (the opponent's trigger order), exiling Hellspark
// Elemental, and the leaves trigger reads the live list at resolution
// (effects context.effectiveSourceRemembered) and returns it. The kernel's
// kept set rode the new incarnation's link (dirac/zone_rememberpersist.go),
// which a version-matched seed skipped: the card stayed exiled.
func TestMBCRemPersistBlinkLiveReadReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/mbc_rempersist_blink_live_read.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("the recording plays pool %s, want custom", r.Pool)
	}
	stdReproNamed(t, "mbc_rempersist_blink_live_read")
}
