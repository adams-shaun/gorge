package oracle

import (
	"testing"
)

// TestMBCBallistaXPayGemSkipReprosLockstep replays the recorded dr-mbc-abbb
// game (union pool, decks focus2_00233_WBR vs focus2_00079_WBR, seed
// 3625844914334216298, pair 59): p1 casts Walking Ballista ({X}{X}) at X = 3
// beside a lucky Gemstone Caverns. Gemstone Caverns is a Condition-split
// source the payment planner defers, so the kernel's lowered-cast payment
// (cast.go bsPay, loweredWindowOK's window walk) took it out of the walk:
// it tapped p1's lands with the Caverns untouched while gorge's CR 601.2g
// window answer tapped the Caverns first, and the kernel's stack spell kept
// X = 3 where gorge's carried none.
func TestMBCBallistaXPayGemSkipReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/mbc_xwin_condsplit.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("the recording plays pool %s, want custom", r.Pool)
	}
	stdReproNamed(t, "mbc_xwin_condsplit")
}
