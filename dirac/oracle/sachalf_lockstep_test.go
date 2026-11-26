package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestSacHalfLockstepMutation proves the recorded Tectonic Split game reaches
// the half-the-lands sacrifice (kop.Cost.SacHalf): the recording replays clean,
// and with the kernel's count rounding down instead of up (a seat with an odd
// number of lands sacrifices one fewer) the same game diverges. Without it a
// green replay could mean the card was never cast.
func TestSacHalfLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "sac_half_cast_sacrifices_half_the_lands.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg, set := synthReg(t), globSet(t, r.Pool, r)
	if res := replayDecodedRes(t, reg, set, r); res.Div != nil {
		t.Fatalf("the recording diverges before the mutation: %s %s %v", res.Div.What, res.Div.Field, res.Div.Diff)
	}
	dirac.DebugSacHalfRoundsDown = true
	defer func() { dirac.DebugSacHalfRoundsDown = false }()
	if res := replayDecodedRes(t, reg, set, r); res.Div == nil {
		t.Fatal("no divergence with the half-sacrifice rounding down: the recording never sacrifices an odd number of lands")
	}
}
