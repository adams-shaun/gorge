package oracle

import (
	"path/filepath"
	"testing"
)

// CR 601.2g-h with CR 117.5/704.3: the free-play tail of the Beanstalk Giant
// lowering recording reaches the target asks FirstPolicy passes over. The
// leading side's lowering and the kernel's own cast build answer the
// Spiteful Prankster pings' target ask the same way (the lowest index, the
// payer's own face), and each resolved pre-cast trigger returns to a
// priority that runs its state-based check, so the payer dies at 0 with the
// third ping unresolved on both sides and the game ends together
// (dr-mbc-bbbb pair 175, decision 466). The recorded game must also replay
// clean when the tail casts instead of passing.
func TestMBCBeanstalkGiantFreePlayTailLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "lowering_yield_self_target.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions, Tail: castFirstPolicy(r.Seed)}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if msg := reproSkipFailure(r, pol); msg != "" {
		t.Fatal(msg)
	}
	if res.Div != nil {
		t.Fatalf("free-play tail diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
