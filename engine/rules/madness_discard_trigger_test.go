package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMadnessExileIsStillADiscard: madness replaces where the discarded card goes,
// not the discard (CR 702.35a), so "whenever you discard" triggers see it.
func TestMadnessExileIsStillADiscard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(`{"name":"madness-discard-trigger","cr":["702.35a"],"why":"madness is still a discard",
"setup":{"p0":{"hand":["Tormenting Voice","Fiery Temper"],"mana":"RR","battlefield":["Marauding Mako"],"library_top":["Island","Island"]},"p1":{}},
"steps":[{"op":"cast","seat":0,"card":"p0:Tormenting Voice","mana":"RR","answers":[{"kind":"any","pick":["p0:Fiery Temper"]}]},
{"op":"resolve"},{"op":"resolve"},{"op":"resolve"}],
"expect":[{"card":"p0:Marauding Mako","counters":{"P1P1":1}},{"card":"p0:Fiery Temper","zone":"graveyard"}]}`), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v\n%s", err, res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
