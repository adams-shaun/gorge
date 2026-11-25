package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// An activate step that names a source's second mana ability by index and
// queues the colour it makes must answer the wheel (which of the source's
// mana abilities) and the colour ask separately. The queued colour answer
// used to own the wheel, so an any-colour ability beside "{T}: Add {C}"
// (Tarnished Citadel, Sliver Hive) failed with `answer "U" not offered for
// choose`: the wheel offers "Add C" and "Add any color", never U. A flattened
// combo (Shivan Reef: "Add C", "Add U", "Add R") keeps taking its colour from
// the wheel itself.
func TestOracleManaStageIndexWithColourAnswer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct{ card, colour string }{
		{"Tarnished Citadel", "U"}, {"Sliver Hive", "G"}, {"Shivan Reef", "R"},
	} {
		sc := fmt.Sprintf(`{"name":"mana-stage-index-colour","cr":["605.3a"],"why":"ability_index picks the wheel, the queued answer the colour","setup":{"p0":{"battlefield":[%q]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"activate","seat":0,"card":"p0:%s","ability_index":1,"answers":[{"kind":"choose","pick":[%q]}]}]}`, c.card, c.card, c.colour)
		res, err := RunOracleScenarioJSON(reg, []byte(sc))
		if err != nil {
			t.Fatalf("%s: %v", c.card, err)
		}
		if len(res.Fails) != 0 {
			t.Errorf("%s: %v\n%s", c.card, res.Fails, strings.Join(res.Transcript, "\n"))
			continue
		}
		if len(res.Snapshots) < 2 {
			t.Errorf("%s: no activate checkpoint: %d snapshots", c.card, len(res.Snapshots))
			continue
		}
		if p := res.Snapshots[1].Players[0].Pool; !strings.Contains(p, c.colour) {
			t.Errorf("%s: step-0 pool = %q, want %s from ability 1", c.card, p, c.colour)
		}
	}
}
