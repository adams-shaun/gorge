package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const dominateRepro = "dominate_x_bound_target_offered_past_a_dual_land"

// CR 601.2b, 601.2c: Dominate ({X}{1}{U}{U}, target creature with mana value X
// or less) is castable only while some X the sources can pay leaves it a legal
// target. The recorded game holds Island, Island and Witherbloom Campus untapped
// with only a mana value 1 and a mana value 3 creature in play: three mana, so X
// is 0 and no creature qualifies. gorge's X-bound target sweep used to price X = 1
// against the potential-mana over-bound (the Campus counted in both of its
// colours) and offer the cast; the X-sweep now bounds it by the payment planner
// on both sides, so the recorded game replays clean and the card is unrefused.
func TestMBCDominateReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", dominateRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), cardset.Raw(printedDelveSet(t, r.Cards)), r, pol)
	// The recording ends on the X = 1 cast both engines used to offer; neither
	// offers it now, so exactly that last action goes unserved.
	if pol.next != len(r.Actions) || pol.Skipped != 1 || pol.firstSkipAt != len(r.Actions)-1 {
		t.Fatalf("recording served %d of %d actions (%d skipped, first at %d): want only the last, the unpayable Dominate cast, skipped", pol.next, len(r.Actions), pol.Skipped, pol.firstSkipAt)
	}
	if !res.Clean() {
		t.Fatalf("Dominate's recorded game no longer replays clean: div %+v, stall %q", res.Div, res.Stall)
	}
}
