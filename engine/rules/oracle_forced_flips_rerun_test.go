package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A forced flip followed by a mid-resolution ask (Fiery Gambit's "flip
// again?", RepeatOptional$) re-executes the resolution from its checkpoint
// once the ask is answered. The re-run must replay the SAME recorded flip,
// not take the next one: forced [heads, tails] with one "yes" is win, flip
// again, lose -- Fiery Gambit has no effect and exactly two flips report.
func TestOracleRunForcedCoinFlipsReExecuted(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"fiery gambit forced flips","cr":["705.1"],"why":"inline",` +
		`"coin_flips":[{"step":0,"seat":0,"win":true,"draws":1},{"step":1,"seat":0,"win":false,"draws":1}],` +
		`"setup":{"p0":{"hand":["Fiery Gambit"],"mana":"RRR"},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Fiery Gambit","mana":"RRR","targets":["p1:Grizzly Bears"]},` +
		`{"op":"resolve","answers":[{"kind":"any","pick":["yes"]}],` +
		`"expect":[{"card":"p1:Grizzly Bears","zone":"battlefield"},{"life":{"p1":20}}]}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("forced [heads, tails] with one yes: %v (flips %+v)", res.Fails, res.Flips)
	}
	if len(res.Flips) != 2 || !res.Flips[0].Win || res.Flips[1].Win {
		t.Fatalf("reported flips %+v, want [win, lose]", res.Flips)
	}
}
