package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestWindowColStartingTownUndecidedAnswersFirstColour pins the Starting Town
// any-colour painland's colour ask when the window's clamp is undecided (a
// hybrid pip, or a generic the pool does not cover yet): the ability lists no
// symbol, so gorge's window offers every colour and the payment window's bot
// takes the first listed one (W), not {C}. The decided case is covered by
// TestWindowOwedCountsTheRestrictedManaOnce.
func TestWindowColStartingTownUndecidedAnswersFirstColour(t *testing.T) {
	t.Parallel()
	sc := newScen(t, Config{})
	sl := sc.slot(sc.perm(v.Forest, 0))
	sc.k.painMana = true
	// A hybrid pip keeps windowUseful undecided; the owed {R} still names the
	// ability in the demand.
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.R: 1}, Hyb: [4]kop.ColorMask{1}}
	pc := &pcard{painCol: kop.ColAny, manaN: 1}
	if got := sc.s.windowCol(sc.k, 0, &cost, sl, pc, false, false); got != int(kop.W) {
		t.Fatalf("undecided any-colour painland answer %d, want W (%d)", got, kop.W)
	}
}
