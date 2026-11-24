package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A lowered scenario carries the live game's coin-flip results
// (coin_flips); the oracle runner forces them instead of flipping from its
// own seed (CR 705.1: the result is random, so only a forced result makes
// the runner agree with the tape). Goblin Bangchuckers: a won flip deals 2
// to the target player, a lost one deals 2 to Bangchuckers itself (a 2/2,
// so it dies). Each forced side must resolve its branch and report itself.
func TestOracleRunForcedCoinFlips(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		win       bool
		life      int32
		graveyard int
	}{
		{win: true, life: 18, graveyard: 0},
		{win: false, life: 20, graveyard: 1},
	} {
		t.Run(fmt.Sprint("win=", tc.win), func(t *testing.T) {
			sc := fmt.Sprintf(`{"name":"bangchuckers forced flip","cr":["705.1"],"why":"inline",`+
				`"coin_flips":[{"step":1,"seat":0,"win":%t,"draws":1}],`+
				`"setup":{"p0":{"battlefield":["Goblin Bangchuckers"]}},"steps":[`+
				`{"op":"activate","seat":0,"card":"p0:Goblin Bangchuckers","targets":["p1"],"ability_index":0},`+
				`{"op":"resolve","expect":[{"life":{"p1":%d},"graveyard_size":{"p0":%d}}]}]}`,
				tc.win, tc.life, tc.graveyard)
			res, err := RunOracleScenarioJSON(reg, []byte(sc))
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(res.Flips) != 1 {
				t.Fatalf("want exactly one reported flip, got %+v (fails %v)", res.Flips, res.Fails)
			}
			if f := res.Flips[0]; f.Seat != 0 || f.Win != tc.win {
				t.Fatalf("reported flip %+v, want seat 0 win %t", f, tc.win)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("forced win=%t did not resolve its branch: %v", tc.win, res.Fails)
			}
		})
	}
}
