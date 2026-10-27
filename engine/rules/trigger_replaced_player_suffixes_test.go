package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTriggerReplacedPlayerSuffixesStandstill: Standstill's "each of that
// player's opponents draws three cards" (Defined$ TriggeredCardOpponent)
// draws for the caster's opponents, not for Standstill's controller.
func TestTriggerReplacedPlayerSuffixesStandstill(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	lib := `"library":["Forest","Forest","Forest","Forest","Forest"]`
	cases := []struct{ name, raw string }{
		{"controller casts", `{"name":"standstill-own","setup":{"p0":{"battlefield":["Standstill","Forest"],"hand":["Grizzly Bears"],` + lib + `},"p1":{` + lib + `}},
		 "steps":[{"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"GC"},{"op":"resolve"}],
		 "expect":[{"count":{"seat":0,"zone":"battlefield","name":"Standstill"},"eq":0},
		  {"count":{"seat":0,"zone":"hand","name":""},"eq":0},{"count":{"seat":1,"zone":"hand","name":""},"eq":3}]}`},
		{"opponent casts", `{"name":"standstill-opp","setup":{"p0":{"battlefield":["Standstill"],` + lib + `},"p1":{"hand":["Lightning Bolt"],"battlefield":["Mountain"],` + lib + `}},
		 "steps":[{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},{"op":"cast","seat":1,"card":"p1:Lightning Bolt","mana":"R","targets":["p0"]},{"op":"resolve"}],
		 "expect":[{"count":{"seat":0,"zone":"battlefield","name":"Standstill"},"eq":0},
		  {"count":{"seat":0,"zone":"hand","name":""},"eq":3},{"count":{"seat":1,"zone":"hand","name":""},"eq":0}]}`},
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
