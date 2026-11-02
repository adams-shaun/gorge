package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// ValidSource$ Ability covers triggered abilities as well as activated ones: Loki,
// God of Mischief draws when Chainsaw's enters trigger targets, in Forge and XMage
// alike (played). A spell is the control.
func TestBecomesTargetAbilitySourceIncludesTriggered(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const setup = `"setup":{"p0":{"battlefield":["Loki, God of Mischief"],"hand":["Chainsaw","Shock"],"library_top":["Island"]},"p1":{"battlefield":["Hill Giant"]}},`
	hand := func(t *testing.T, steps string) int {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(`{"name":"loki-source","why":"an ability targets",`+setup+`"steps":[`+steps+`]}`))
		if err != nil || len(res.Fails) != 0 {
			t.Fatalf("err=%v fails=%v", err, res.Fails)
		}
		return len(res.Snapshots[len(res.Snapshots)-1].Players[0].Hand)
	}
	if n := hand(t, `{"op":"cast","seat":0,"card":"p0:Chainsaw","mana":"CR"},{"op":"resolve","answers":[{"kind":"target","pick":["p1:Hill Giant"]}]}`); n != 2 {
		t.Fatalf("a triggered ability targeted: hand %d, want Shock and the card drawn", n)
	}
	if n := hand(t, `{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1:Hill Giant"]},{"op":"resolve"}`); n != 1 {
		t.Fatalf("a spell targeted: hand %d, want Chainsaw alone", n)
	}
}
