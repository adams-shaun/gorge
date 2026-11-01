package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Wild Slash's one target rides a chain link, and it copies per candidate like a
// spell whose root targets (CR 707.10d).
func TestCopyForEachCandidateOfAChainLinkTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct{ name, setup, steps, expect string }{
		{
			name:   "Zada copies it for each other creature",
			setup:  `{"p0":{"battlefield":["Zada, Hedron Grinder","Grizzly Bears","Hill Giant"],"hand":["Wild Slash"],"mana":"R"},"p1":{"battlefield":["Craw Wurm"]}}`,
			steps:  `{"op":"cast","seat":0,"card":"p0:Wild Slash","targets":["p0:Zada, Hedron Grinder"],"mana":"R"},{"op":"resolve"}`,
			expect: `[{"card":"p0:Grizzly Bears","zone":"graveyard"},{"card":"p0:Hill Giant","damage":2},{"card":"p0:Zada, Hedron Grinder","damage":2},{"card":"p1:Craw Wurm","damage":0}]`,
		},
		{
			name:   "Zada doesn't copy one aimed elsewhere",
			setup:  `{"p0":{"battlefield":["Zada, Hedron Grinder","Hill Giant"],"hand":["Wild Slash"],"mana":"R"},"p1":{}}`,
			steps:  `{"op":"cast","seat":0,"card":"p0:Wild Slash","targets":["p0:Hill Giant"],"mana":"R"},{"op":"resolve"}`,
			expect: `[{"card":"p0:Zada, Hedron Grinder","damage":0},{"card":"p0:Hill Giant","damage":2}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := `{"name":"copy link","cr":["707.10d"],"why":"copy","setup":` + tc.setup + `,"steps":[` + tc.steps + `],"expect":` + tc.expect + `}`
			res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
			if err != nil || len(res.Fails) != 0 || res.ReplayDiff != "" {
				t.Fatalf("err=%v fails=%v replay=%q\n%s", err, res.Fails, res.ReplayDiff, res.Transcript)
			}
		})
	}
}
