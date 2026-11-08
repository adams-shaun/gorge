package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The same pair on the autopay surface is clean. That surface offers no plain
// cast gorge's planner never prices, so Duskwatch Hunter's hybrid cast is not
// lowered and the Treasure that funds it is never spent at priority: the
// pre-cast drain of The Sackville-Bagginses never runs, and the game goes on
// the ordinary payment path. This pins the full-surface state-based check
// (resolveLowerCounterTriggers) to the surface it belongs to.
func TestLoweredHybridCastLethalSacrificeAutopayLockstep(t *testing.T) {
	t.Parallel()
	const name = "lowered_hybrid_cast_lethal_sacrifice_trigger_autopay"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Surface != "autopay" {
		t.Fatalf("fixture surface %q, want autopay", r.Surface)
	}
	// The shape's two cards are in the first seat's deck: this pins the
	// hybrid cast beside The Sackville-Bagginses, not some other game.
	var dusk, sack bool
	for _, c := range r.Cards[0] {
		dusk = dusk || c == "Duskwatch Hunter"
		sack = sack || c == "The Sackville-Bagginses"
	}
	if !dusk || !sack {
		t.Fatalf("seat 0's deck lacks the shape: Duskwatch Hunter %v, The Sackville-Bagginses %v", dusk, sack)
	}
	replayRepro(t, synthReg(t), cardsettest.Synth(t, "std-hob"), name, true, false)
}
