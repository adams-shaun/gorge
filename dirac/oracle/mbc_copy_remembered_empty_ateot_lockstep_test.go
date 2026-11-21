package oracle

import "testing"

// TestMBCSunscourgeChampionReprosLockstep replays the synthetic tail-53 game
// the evidence loop recorded: Mordor on the March ("create a token that's a
// copy of it ... Exile it at the beginning of the next end step") is cast with
// storm while its controller's graveyard holds no creature card, so the
// instruction remembers nothing and makes no copy. The AtEOT$ rider still
// creates its delayed triggered ability (CR 603.7), which fires at the end
// step and does nothing; gorge registers it (copypermanent.go, the Remembered
// arm, scheduleAtEOTNone) and the two resolutions' triggers are ordered at the
// end step (decision 262: trigger_order against the kernel's priority). The
// kernel's execCopyToken returned before registering one.
func TestMBCSunscourgeChampionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_remembered_empty_ateot_rider")
}
