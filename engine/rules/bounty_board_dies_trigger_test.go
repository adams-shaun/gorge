package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBountyBoardDiesTrigger: Bounty Board's "Whenever a creature with a
// bounty counter on it dies, each of its controller's opponents draws a card
// and gains 2 life" names the dying creature's controller's opponents (CR
// 603.10a: a leaves-the-battlefield trigger looks back in time, so "its
// controller" is the creature's last-known controller). Forge spells that
// Defined$ Player.OpponentOf TriggeredCardController; gorge must resolve it
// the same way it resolves TriggeredCardOpponent.
func TestBountyBoardDiesTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	lib := `"library":["Forest","Forest","Forest","Forest","Forest"]`
	cases := []struct{ name, raw string }{
		// p1's Memnite carries the bounty; p0 destroys it, so p0 gains 2 and
		// draws 1. p1 (the trigger's controller) gets nothing.
		{"opponent's creature dies", `{"name":"bounty-board-opp","setup":{"p0":{"hand":["Murder"],"battlefield":["Swamp","Swamp","Swamp"],` + lib + `},
			 "p1":{"battlefield":["Bounty Board","Memnite"],"counters":{"Memnite":{"BOUNTY":1}},` + lib + `}},
			 "steps":[{"op":"cast","seat":0,"card":"p0:Murder","mana":"CBB","targets":["p1:Memnite"]},{"op":"resolve"}],
			 "expect":[{"life":{"p0":22,"p1":20}},
			  {"count":{"seat":0,"zone":"hand","name":""},"eq":1},{"count":{"seat":1,"zone":"hand","name":""},"eq":0}]}`},
		// p0's Memnite carries the bounty; p0 destroys it, so p1 (p0's
		// opponent) gains 2 and draws 1, and p0 gets nothing.
		{"own creature dies", `{"name":"bounty-board-own","setup":{"p0":{"hand":["Murder"],"battlefield":["Swamp","Swamp","Memnite"],"counters":{"Memnite":{"BOUNTY":1}},` + lib + `},
			 "p1":{"battlefield":["Bounty Board"],` + lib + `}},
			 "steps":[{"op":"cast","seat":0,"card":"p0:Murder","mana":"CBB","targets":["p0:Memnite"]},{"op":"resolve"}],
			 "expect":[{"life":{"p0":20,"p1":22}},
			  {"count":{"seat":0,"zone":"hand","name":""},"eq":0},{"count":{"seat":1,"zone":"hand","name":""},"eq":1}]}`},
	}
	for _, tc := range cases {
		sc, err := decodeOracleScenario([]byte(tc.raw))
		if err != nil {
			t.Fatal(err)
		}
		fails, transcript, _ := runOracleScenario(reg, sc)
		if len(fails) != 0 {
			t.Errorf("%s: %s\n%s", tc.name, strings.Join(fails, "\n"), strings.Join(transcript, "\n"))
		}
	}
}
