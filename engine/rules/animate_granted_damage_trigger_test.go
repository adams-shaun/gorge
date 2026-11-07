package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAnimateGrantedDamageTriggerKeepsOtherObserversTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	raw := `{"name": "ire_ball", "turn": 1, "cr": [], "why": "x", "setup": {"p0": {"battlefield": ["Mountain", "Hill Giant"], "hand": ["Dreadmaw's Ire"], "library_top": ["Island", "Island", "Island", "Island", "Island"], "life": 20}, "p1": {"battlefield": ["Contested Game Ball", "Ornithopter"], "hand": [], "library_top": ["Island", "Island", "Island", "Island", "Island"], "life": 20}}, "steps": [{"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "attack", "seat": 0, "attackers": ["p0:Hill Giant"], "defender": "p1"}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "block", "seat": 1}, {"op": "activate", "seat": 0, "card": "p0:Mountain", "ability_index": 0}, {"op": "cast", "seat": 0, "card": "p0:Dreadmaw's Ire", "targets": ["p0:Hill Giant"]}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}], "expect": []}`
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range res.Transcript {
		t.Log(l)
	}
	t.Logf("%+v", res.Fails)
	for _, d := range res.Decisions {
		t.Logf("DEC %+v", d)
	}
	t.Logf("LAST %+v", res.Snapshots[len(res.Snapshots)-1].Stack)
}
