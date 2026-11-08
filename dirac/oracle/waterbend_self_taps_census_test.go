package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The pair-328 fixture exists for the waterbend announcement at decision 484:
// Great Divide Guide's granted {T} on Compassionate Healer fires the Healer's
// own Taps watcher, which gorge's census cannot price, so the announcement
// offers Done. The fixed game then runs on to the declared maxDProg capacity,
// so the old stdRepro row's stall:true accepted a stall anywhere; assert the
// exact declared stop instead.
func TestWaterbendSelfTapsWatcherCensusStopsAtTheDeclaredDProgCapacity(t *testing.T) {
	t.Parallel()
	res := replayReproRes(t, synthReg(t), cardsettest.Synth(t, "std-tla"), "waterbend_self_taps_watcher_census", false)
	want := KernelStallPrefix + "dprog"
	if res.Stall != want {
		t.Fatalf("stall %q at %d decisions, want %q (%d): a regression must not end the replay before the waterbend menu the fixture covers", res.Stall, res.Decisions, want, 542)
	}
	if res.Decisions != 542 {
		t.Fatalf("declared capacity reached after %d decisions, want 542 (the fixed menu is at 484)", res.Decisions)
	}
}
