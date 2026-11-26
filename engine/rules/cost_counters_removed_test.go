package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCostCountersRemovedReadsThePaidPile pins SVar$CostCountersRemoved, the
// counters a SubCounter<All/KIND> cost removed (Forge's CostRemoveCounter
// sets the ability's CostCountersRemoved SVar to the amount it removed).
// gorge read the unknown name as zero, so Molten Hydra dealt 0 and Essence
// Bottle gained 0. CR 602.2b, 601.2h: the cost is paid on activation; the
// effect then reads how many counters were removed "this way". Molten Hydra
// with three +1/+1 counters deals 3; Essence Bottle with two elixir counters
// gains 4 (X:SVar$CostCountersRemoved/Twice); Relic Amulet with none deals 0.
func TestCostCountersRemovedReadsThePaidPile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	raw := `{"name":"cost counters removed","turn":1,"cr":["602.2b","601.2h"],"why":"SVar$CostCountersRemoved is the counters the SubCounter<All> cost removed",
	 "setup":{"p0":{"battlefield":["Molten Hydra","Essence Bottle"],"counters":{"Molten Hydra":{"P1P1":3},"Essence Bottle":{"ELIXIR":2}},"library":["Plains","Plains","Plains"],"life":20},"p1":{"battlefield":["Plains"],"life":20}},
	 "steps":[
	  {"op":"activate","seat":0,"card":"p0:Molten Hydra","ability_index":1,"targets":["p1"],"expect":[{"card":"p0:Molten Hydra","tapped":true,"counters":{"P1P1":0}}]},
	  {"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"life":{"p0":20,"p1":17}}]},
	  {"op":"activate","seat":0,"card":"p0:Essence Bottle","ability_index":1,"expect":[{"card":"p0:Essence Bottle","tapped":true,"counters":{"ELIXIR":0}}]},
	  {"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"life":{"p0":24,"p1":17}}]}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
}
