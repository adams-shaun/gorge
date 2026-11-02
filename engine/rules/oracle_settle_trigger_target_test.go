package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A checkpoint waits for the target of a trigger going on the stack, as Forge and
// XMage do inside the pass: unanswered, the opponent's trigger from the same deaths
// is not on the stack yet, and the two engines read as apart when they are not.
func TestOracleCheckpointSettlesATriggerTargetAsk(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(`{"name":"trigger-target-at-a-checkpoint","why":"simultaneous dies triggers with targets",
"setup":{"p0":{"battlefield":["Bile-Vial Boggart"]},"p1":{"battlefield":["Dynamite Diver"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Bile-Vial Boggart"],"defender":"p1"},{"op":"pass","seat":0},{"op":"pass","seat":1},
{"op":"block","seat":1,"blocks":[["p1:Dynamite Diver","p0:Bile-Vial Boggart"]]},{"op":"pass","seat":0},{"op":"pass","seat":1}]}`))
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v", err, res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if len(last.Stack) != 2 || last.Stack[0].Controller != 1 || last.Stack[1].Controller != 0 {
		t.Fatalf("at the damage checkpoint the stack is %+v, want the opponent's trigger over the active player's", last.Stack)
	}
}
