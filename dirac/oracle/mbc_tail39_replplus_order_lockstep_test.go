package oracle

import (
	"path/filepath"
	"testing"
)

// The other legal CR 616.1 order of the tail39_replplus game (the recorded
// replay's first tail decision, 641): the source controller's Absorbing Man
// and Titania doublings first, the recipient's Tok-Tok, Volcano Born plus
// second. The first fix collected the recipient's plus candidate
// (replOrderCandsSeat scans every seat's battlefield), but the walk's live
// check replCandLive still gated both the plus and the doubling arms on the
// source's controller, so the last-remaining recipient plus was dropped and
// dirac dealt 2n+1 (5) where gorge deals 2(n+1) (6): player 0's life 5
// against 4.
func TestMBCTail39ReplplusOrderLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "tail39_replplus.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	asked := false
	askSeq, askOpts := -1, 0
	pol := &ScriptPolicy{Actions: r.Actions, Tail: PolicyFunc(func(c *PickCtx) Canon {
		if asked || len(c.D.Options) < 2 {
			return c.D.Options[0]
		}
		asked = true // the first tail decision with a choice: the order ask
		askSeq, askOpts = c.Seq, len(c.D.Options)
		return c.D.Options[1]
	})}
	reg := synthReg(t)
	set := globSet(t, r.Pool, r)
	res := replayDecodedScript(t, reg, set, r, pol)
	if !asked {
		t.Fatal("the replay never reached a tail decision with two or more options: no order ask was posed")
	}
	// Pin the ask the latch took: the first tail decision, right after the
	// recorded actions. A re-recording that adds an earlier multi-option tail
	// decision moves it and must fail here rather than quietly test the
	// plus-first order through FirstPolicy.
	if askSeq != len(r.Actions) {
		t.Fatalf("the latch took decision %d (%d options), want the order ask at %d", askSeq, askOpts, len(r.Actions))
	}
	if msg := reproSkipFailure(r, pol); msg != "" {
		t.Fatal(msg)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
