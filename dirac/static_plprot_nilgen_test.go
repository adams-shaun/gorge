package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlayerTargetHandWrittenTable: a hand-written table (no generated
// vocabulary, so prep.gen is nil) offers a player as a legal TgtAny and
// TgtPlayer target; the player-protection probe must be nil-safe.
func TestPlayerTargetHandWrittenTable(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	if sc.k.gen != nil {
		t.Fatal("precondition: vanilla's table must lower to a nil gen")
	}
	for _, kind := range []kop.TargetKind{kop.TgtAny, kop.TgtPlayer} {
		spec := kop.TargetSpec{Kind: kind}
		for p := uint8(0); p < 2; p++ {
			if !sc.s.targetOK(sc.k, &spec, 0, NoSlot, NoObj, TCode(PlayerCode(p))) {
				t.Errorf("kind %v: player %d rejected as a target", kind, p)
			}
		}
	}
}
