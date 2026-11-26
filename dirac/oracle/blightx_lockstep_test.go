package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestBlightXLockstepMutation proves the recorded Soul Immolation game reaches
// the announced Blight<X> cast (kop.CostBlight with Cost.N 0): the recording
// replays clean, and with the kernel's X ask no longer bounded by the greatest
// toughness among the caster's creatures (gorge xAsk's blightX cap, `XMax$ GrTo`)
// the same game diverges at the X menu. Without it a green replay could mean the
// card was never cast.
func TestBlightXLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "blight_x_cast_bounds_x_by_greatest_toughness.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg, set := synthReg(t), globSet(t, r.Pool, r)
	if res := replayDecodedRes(t, reg, set, r); res.Div != nil {
		t.Fatalf("the recording diverges before the mutation: %s %s %v", res.Div.What, res.Div.Field, res.Div.Diff)
	}
	dirac.DebugBlightXCapIgnored = true
	defer func() { dirac.DebugBlightXCapIgnored = false }()
	if res := replayDecodedRes(t, reg, set, r); res.Div == nil {
		t.Fatal("no divergence with the X ask unbounded: the recording never announces a Soul Immolation X")
	}
}
