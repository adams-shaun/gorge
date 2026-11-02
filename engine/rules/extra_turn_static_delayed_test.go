package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A delayed trigger marked Static$ True that an extra turn carries runs off the
// stack when it fires, as one registered directly does (Forge's isStatic arm,
// read; Forge and XMage both show Kang's upkeep with an empty stack, played). Final
// Fortune's end-step trigger is the control: not static, so it uses the stack.
func TestExtraTurnStaticDelayedStaysOffTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	stackAt := func(t *testing.T, step string, setup, first string) int {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(`{"name":"extra-turn-delayed","why":"a delayed trigger on an extra turn",
"setup":{"p0":{`+setup+`,"library_top":["Island","Island","Island"]},"p1":{"library_top":["Island","Island","Island"]}},
"steps":[`+first+`,{"op":"resolve"},{"op":"pass_to","step":"upkeep","active":"p0"},{"op":"pass_to","step":"`+step+`","active":"p0"}]}`))
		if err != nil || len(res.Fails) != 0 {
			t.Fatalf("err=%v fails=%v", err, res.Fails)
		}
		last := res.Snapshots[len(res.Snapshots)-1]
		if last.Step != step || last.Active != 0 {
			t.Fatalf("stopped at %s, active seat %d", last.Step, last.Active)
		}
		return len(last.Stack)
	}
	if n := stackAt(t, "upkeep", `"battlefield":["Kang the Conqueror"]`,
		`{"op":"activate","seat":0,"card":"p0:Kang the Conqueror","mana":"CCCCCUUU","ability_index":0}`); n != 0 {
		t.Fatalf("Kang's static delayed trigger: %d on the stack, want 0", n)
	}
	if n := stackAt(t, "end", `"hand":["Final Fortune"]`,
		`{"op":"cast","seat":0,"card":"p0:Final Fortune","mana":"RR"}`); n != 1 {
		t.Fatalf("Final Fortune's delayed trigger: %d on the stack, want 1", n)
	}
}
