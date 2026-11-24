package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmJarOfEyeballs pins the SUP farm window SUP-s338-supp
// xe:RAND:0:19:w17: p0 activates Jar of Eyeballs ("{3}, {T}, Remove all
// eyeball counters from Jar of Eyeballs", Cost$ 3 T SubCounter<All/EYEBALL>).
// Forge and XMage both spend exactly {3} and leave one B of BBBB floating;
// gorge priced the SubCounter<All/...> token as an unknown symbol plus one
// phantom generic and spent all four. CR 602.2b/601.2f: the total cost is the
// printed cost plus live increases only; CR 118.3: removing counters is a
// non-mana cost. Zero counters is a legal payment of "remove all".
func TestFarmJarOfEyeballs(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name     string
		counters int
		mana     string
		offered  bool
		left     string
	}{
		{"exact three with counters", 3, "BBB", true, ""},
		{"four leaves one", 3, "BBBB", true, "B"},
		{"two is not enough", 3, "BB", false, ""},
		{"zero counters still pays three", 0, "BBB", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			counters := ""
			if tc.counters > 0 {
				counters = fmt.Sprintf(`,"counters":{"Jar of Eyeballs":{"EYEBALL":%d}}`, tc.counters)
			}
			steps := fmt.Sprintf(`{"op":"mana","seat":0,"mana":%q,"expect":[`+
				`{"card":"p0:Jar of Eyeballs","counters":{"EYEBALL":%d}},`+
				`{"offered":{"seat":0,"kind":"activate","card":"p0:Jar of Eyeballs"},"want":%t}]}`,
				tc.mana, tc.counters, tc.offered)
			if tc.offered {
				steps += fmt.Sprintf(`,{"op":"activate","seat":0,"card":"p0:Jar of Eyeballs","ability_index":0,"expect":[`+
					`{"pool":{"p0":%q}},{"card":"p0:Jar of Eyeballs","tapped":true,"counters":{"EYEBALL":0}}]}`, tc.left)
			}
			raw := fmt.Sprintf(`{"name":%q,"turn":1,
			 "why":"CR 602.2b/601.2f, CR 118.3: Remove all eyeball counters adds no mana; Forge and XMage spend {3}",
			 "setup":{"p0":{"battlefield":["Jar of Eyeballs"],"library":["Plains","Plains","Plains","Plains"]%s},"p1":{"battlefield":["Plains"]}},
			 "steps":[%s]}`, t.Name(), counters, steps)
			res, err := RunOracleScenarioJSON(reg, []byte(raw))
			if err != nil {
				t.Fatalf("scenario JSON: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%v", res.Fails)
			}
		})
	}
}
