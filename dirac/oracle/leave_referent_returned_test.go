package oracle

import (
	"slices"
	"testing"
)

// TestLeaveReferentReturnedLockstep replays a dr-mbc-baab game in which
// Settle Beyond Reality's flicker mode exiles its controller's God-Eternal
// Oketra and immediately returns it to the battlefield; the God's
// "dies or is put into exile from the battlefield, you may put it into its
// owner's library third from the top" trigger fires on the exile and its
// optional decider answers yes, but the card is back on the battlefield by
// the time the ability resolves. A leaves-the-battlefield ability checks for
// the card only in the first zone it went to (CR 603.6c): the kernel must
// leave it there, as gorge's moved-since-fire drop does, instead of moving it
// into the library ("If an effect exiles the God and immediately returns it
// to the battlefield, its last ability triggers but will have no effect",
// the cycle's 2019-05-03 ruling).
func TestLeaveReferentReturnedLockstep(t *testing.T) {
	t.Parallel()
	// the preconditions: the flicker and the God are in the same deck, so the
	// trigger's referent is the card the flicker returned, and the compared
	// library states differ without the kernel's no-effect gate
	r, err := ReadRepro("testdata/repro/leave_referent_returned.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.Cards[0], "God-Eternal Oketra") {
		t.Fatal("deck 0 holds no God-Eternal Oketra")
	}
	if !slices.Contains(r.Cards[0], "Settle Beyond Reality") {
		t.Fatal("deck 0 holds no Settle Beyond Reality")
	}
	res, skipped := synthReproRun(t, "repro/leave_referent_returned", "dr-mbc-baab", Config{GorgeAutopaySurface: true})
	if skipped != 0 {
		t.Errorf("%d scripted actions were not offered", skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if len(res.Actions) < 385 {
		t.Fatalf("the replay ended after %d decisions, before the God's trigger resolved at 385", len(res.Actions))
	}
}
