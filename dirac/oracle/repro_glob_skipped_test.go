package oracle

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlobReplayFailsOnSkippedActions pins the glob runner's guard on script
// service (#832: a drifted recording skipped 322 recorded actions and the
// glob lockstep replay stayed green for a while). A shifted script is
// replayed through the runner's own path: first the precondition — the shift
// skips an action while the game still replays clean, the exact shape that
// hid the drift — then the runner's skip guard must fail it, naming the count
// and the first skipped decision.
func TestGlobReplayFailsOnSkippedActions(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "mbc_split_card_hand_colors.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Actions) < 2 {
		t.Fatalf("the fixture has %d recorded actions, want at least 2 to shift", len(r.Actions))
	}
	shifted := *r
	// The shift: duplicate the first action at the front. Decision 0 serves
	// the copy, and the original is then offered at a later decision that does
	// not offer it, so the script skips it even though the game path never
	// moves.
	shifted.Actions = append([]Canon{r.Actions[0]}, r.Actions...)
	pol := &ScriptPolicy{Actions: shifted.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, shifted.Pool, &shifted), &shifted, pol)
	if pol.Skipped == 0 {
		t.Fatal("the shifted fixture skipped no action: no drift to catch")
	}
	if res.Div != nil {
		t.Fatalf("the shifted fixture diverged (%s at %d): the skip guard would not be the only thing that fails it", res.Div.What, res.Div.At)
	}
	// The guard replayReproValue runs before its divergence check must fail
	// the shifted replay: it names the skip count and the first skipped
	// decision, so a drifted recording is loud about what moved.
	msg := reproSkipFailure(&shifted, pol)
	switch {
	case msg == "":
		t.Fatalf("the glob runner's skip guard passed a replay that skipped %d of %d recorded actions", pol.Skipped, len(shifted.Actions))
	case !strings.Contains(msg, fmt.Sprintf("replay skipped %d of %d recorded actions (first at decision", pol.Skipped, len(shifted.Actions))):
		t.Fatalf("the skip guard's failure does not name the count and the first skipped decision: %q", msg)
	}
}
