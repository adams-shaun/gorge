package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.23a: a search finds every card in the zone with the stated quality.
// Wild Pair's "same total power and toughness" is Creature.totalPT_EQX with
// X = the entered creature's power plus toughness (TriggeredCard$CardSumPT).
func wildPairRun(t *testing.T, library, choose string) (lions int, fails []string, transcript string) {
	t.Helper()
	answers := `{"kind":"trigger_optional","pick":["yes"]}`
	if choose != "" {
		answers += `,{"kind":"choose","pick":["` + choose + `"]}`
	}
	sc := `{"name":"wild-pair-total-pt","cr":["701.23a"],"why":"Wild Pair offers a library creature with the same total P/T as the cast creature",
 "setup":{"p0":{"battlefield":["Wild Pair","Plains","Plains"],"hand":["Silvercoat Lion"],"library":[` + library + `]},"p1":{}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Silvercoat Lion","mana":"CW"},{"op":"resolve","answers":[` + answers + `]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(sc))
	if err != nil {
		t.Fatalf("scenario: %v", err)
	}
	for _, p := range czOptionalLast(res).Permanents {
		if strings.Contains(p.Ref, "Silvercoat Lion") {
			lions++
		}
	}
	return lions, res.Fails, strings.Join(res.Transcript, "\n")
}

func TestWildPairFindsACreatureWithTheSameTotalPT(t *testing.T) {
	t.Run("same total is offered and enters", func(t *testing.T) {
		n, fails, tr := wildPairRun(t, `"Silvercoat Lion"`, "Silvercoat Lion")
		if len(fails) != 0 || n != 2 {
			t.Fatalf("Silvercoat Lions on the battlefield = %d (fails %v), want 2\n%s", n, fails, tr)
		}
	})
	t.Run("a 3/3 is not offered for a 2/2", func(t *testing.T) {
		n, fails, tr := wildPairRun(t, `"Hill Giant"`, "Hill Giant")
		if len(fails) == 0 {
			t.Fatalf("picking Hill Giant (3/3) was accepted for a 2/2 Lion; want it not offered\n%s", tr)
		}
		if n != 1 {
			t.Fatalf("Silvercoat Lions = %d, want 1\n%s", n, tr)
		}
	})
}
