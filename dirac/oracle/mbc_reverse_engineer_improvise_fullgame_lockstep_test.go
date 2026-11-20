package oracle

import (
	"path/filepath"
	"testing"
)

// TestMBCReverseEngineerImproviseFullGameLockstep replays the tail-62 pair 55
// recording (CR 702.126a, 601.2h) past its scripted prefix with the policy the
// fuzz run that found it used: random choices with affordable targets. The
// recording's last action is decision 441; the existing prefix test's tail
// passes at 442, where the intermediate tree cast Reverse Engineer, so its
// game from there is not that one. At HEAD both sides decline the credited
// Reverse Engineer cast (kw_improvisepay.go improviseSourcesPay), and the
// full game must agree through decision 451, where a plan paying the
// A-Find the Path grant first differed. The full game under that policy is
// 538 decisions long; the check only requires crossing the old point.
func TestMBCReverseEngineerImproviseFullGameLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "reverse_engineer_improvise.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions, Tail: AffordableTargetPolicy(RandomPolicy(r.Seed))}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", pol.next, len(r.Actions), pol.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	// The game must run past the prefix, or "no divergence" would only say the
	// recorded actions still replay. Decision 451 is where the intermediate
	// granted-source payment first differed.
	if res.Decisions <= 451 {
		t.Fatalf("game ended after %d decisions, before the former divergence at 451", res.Decisions)
	}
}
