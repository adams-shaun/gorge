package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runVigorMortis casts Vigor Mortis paying `mana` over a Silvercoat Lion in
// the graveyard, with the given expectations (a JSON array body).
func runVigorMortis(t *testing.T, name, mana, expect string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"` + name + `","cr":["601.2h"],"why":"Vigor Mortis, ` + mana + ` spent","setup":{"p0":{"hand":["Vigor Mortis"],"graveyard":["Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Vigor Mortis","mana":"` + mana + `","targets":["p0:Silvercoat Lion"]},{"op":"resolve","seat":0}],"expect":` + expect + `}`
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
	}
}

// TestVigorMortisGreenAddsCounter: {G} spent to cast Vigor Mortis
// (ConditionManaSpent$ G) returns the creature with a +1/+1 counter.
func TestVigorMortisGreenAddsCounter(t *testing.T) {
	t.Parallel()
	runVigorMortis(t, "vm-g", "GGBB", `[{"card":"p0:Silvercoat Lion","counters":{"P1P1":1}},{"card":"p0:Silvercoat Lion","pt":"3/3"}]`)
}

// TestVigorMortisNoGreenNoCounter: with no {G} spent (ConditionManaNotSpent$ G)
// the creature still returns, without the counter.
func TestVigorMortisNoGreenNoCounter(t *testing.T) {
	t.Parallel()
	runVigorMortis(t, "vm-b", "BBBB", `[{"card":"p0:Silvercoat Lion","pt":"2/2"}]`)
}
