package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A battlefield GRANT of an alternative cost (Fist of Suns' "ValidSA$ Spell |
// ValidPlayer$ You | Cost$ W U B R G", Rooftop Storm's Zombie-creature {0})
// names no EffectZone$: it functions on the battlefield only (CR 113.6). The
// face-static walk of alternativeCosts read it on the card in hand too, so a
// Fist of Suns in hand offered a {W}{U}{B}{R}{G} cast of ITSELF and a Rooftop
// Storm in hand a free cast of itself, a grant to other spells. Neither is
// offered an alternative cast from the hand; with the grants on the
// battlefield, a Zombie creature spell is (the one list entry per grant).
func TestAlternativeCostGrantNotOfferedFromHand(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	run := func(setup, expect string) []string {
		t.Helper()
		sc := `{"name":"grant-hand","cr":["113.6","118.9"],"why":"inline","setup":{"p0":{` + setup + `}},"steps":[` +
			`{"op":"mana","seat":0,"mana":"WUBRG","expect":[` + expect + `]}]}`
		res, err := RunOracleScenarioJSON(reg, []byte(sc))
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return res.Fails
	}
	offered := func(card string, want bool) string {
		w := "false"
		if want {
			w = "true"
		}
		return `{"offered":{"seat":0,"kind":"cast","card":"p0:` + card + `","label":"(alternative cost)"},"want":` + w + `}`
	}
	if fails := run(`"hand":["Fist of Suns","Rooftop Storm"]`, offered("Fist of Suns", false)+`,`+offered("Rooftop Storm", false)); len(fails) != 0 {
		t.Errorf("a grant card in hand offered an alternative cast of itself: %v", fails)
	}
	// the precondition: the same label is offered once the grant is on the battlefield
	if fails := run(`"battlefield":["Fist of Suns"],"hand":["Walking Corpse"]`, offered("Walking Corpse", true)); len(fails) != 0 {
		t.Errorf("Fist of Suns on the battlefield did not offer the alternative cast: %v", fails)
	}
}
