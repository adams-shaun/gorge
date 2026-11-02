package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// One ChangeZone moving two targets is one event for a "one or more" trigger,
// whatever the zones: Forge and XMage both put one Soul Enervation trigger on the
// stack (played), and one target is the control.
func TestChangesZoneAllFiresOnceForAMultiTargetMove(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		targets string
		want    int
	}{{`["p0:Grizzly Bears","p0:Hill Giant"]`, 1}, {`["p0:Grizzly Bears"]`, 1}} {
		res, err := RunOracleScenarioJSON(reg, []byte(`{"name":"leave-graveyard-batch","why":"two creature cards leave a graveyard together",
"setup":{"p0":{"battlefield":["Soul Enervation"],"hand":["Macabre Reconstruction"],"graveyard":["Grizzly Bears","Hill Giant"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Macabre Reconstruction","mana":"CCCCCB","targets":`+tc.targets+`},{"op":"pass","seat":0},{"op":"pass","seat":1}]}`))
		if err != nil || len(res.Fails) != 0 {
			t.Fatalf("%s: err=%v fails=%v", tc.targets, err, res.Fails)
		}
		if n := len(res.Snapshots[len(res.Snapshots)-1].Stack); n != tc.want {
			t.Fatalf("%s: %d triggers on the stack, want %d", tc.targets, n, tc.want)
		}
	}
}
