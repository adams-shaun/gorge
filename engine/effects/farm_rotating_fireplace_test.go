package effects_test

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm windows SUP-s353-supp xe:RAND:2:0:w11 (and five more, field p0.pool,
// compliance verdict wrong=G): Rotating Fireplace's "{T}: Add an amount of
// {C} equal to the number of time counters on Rotating Fireplace" left
// gorge's pool empty where Forge, the corroborating engine, held one {C} per
// time counter. The script reads X as `Count$CardCounters.Time` while its
// own ETB places `CounterType$ TIME`; Forge resolves counter kinds
// case-insensitively (CounterEnumType.getType upper-cases the name), so both
// spellings are the one TIME counter. CR 106.4: an effect that adds mana puts
// it in the mana pool -- here {C} per time counter.
func TestFarmRotatingFireplace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		extra int32
		want  string
	}{{0, "C"}, {1, "CC"}} {
		t.Run(fmt.Sprintf("time%d", tc.extra+1), func(t *testing.T) {
			counters := ""
			if tc.extra > 0 {
				counters = fmt.Sprintf(`, "counters": {"Rotating Fireplace": {"TIME": %d}}`, tc.extra)
			}
			scenario := fmt.Sprintf(`{
			  "name": "farm-rotating-fireplace",
			  "setup": {"p0": {"battlefield": ["Rotating Fireplace"]%s}},
			  "steps": [
			    {"op": "activate", "seat": 0, "card": "p0:Rotating Fireplace", "ability_index": 0,
			     "expect": [{"card": "p0:Rotating Fireplace", "tapped": true, "counters": {"TIME": %d}},
			                {"pool": {"p0": %q}}]}
			  ]
			}`, counters, tc.extra+1, tc.want)
			res, err := rules.RunOracleScenarioJSON(reg, []byte(scenario))
			if err != nil {
				t.Fatalf("scenario JSON: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%v", res.Fails)
			}
		})
	}
}
