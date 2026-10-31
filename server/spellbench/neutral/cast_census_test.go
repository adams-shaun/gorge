package neutral_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

// TestAutopayCastCensusKeepsThePlannedCast: on an engine_autopay host, Crystal
// Golem looks affordable to the bot (Cascading Cataracts' mana ability reads
// as five of each colour) though the planner offers it no plan. Only the cast
// census says no tap can cast it. Without the census the bot takes the manual
// answer for the Golem, whose taps this host does not offer, and passes with
// Ainok Bond-Kin's plan in hand.
func TestAutopayCastCensusKeepsThePlannedCast(t *testing.T) {
	reg := testcorpus.Registry(t)
	a, err := agent.New("bot")
	if err != nil {
		t.Fatal(err)
	}
	a.SetRegistry(reg)
	if err := a.EnableNeutral(); err != nil {
		t.Fatal(err)
	}
	deck := `{"name":"Mix","decklist":[{"name":"Ainok Bond-Kin","count":4},{"name":"Crystal Golem","count":4},{"name":"Cascading Cataracts","count":4},{"name":"Plains","count":48}]}`
	for _, line := range []string{
		`{"request_type":"hello","protocol":"spellbench/v2","request_id":"r-0"}`,
		`{"request_type":"game_start","protocol":"spellbench/v2","request_id":"r-1","game_id":"g","seat":"p0","agent_seed":7,` +
			`"engine_profile":{"engine_defaults":{"mana_payment":"engine_autopay"}},"own_deck":` + deck + `,"opponent_deck":` + deck + `}`,
	} {
		if out := a.Handle([]byte(line)); strings.Contains(string(out), `"error"`) {
			t.Fatalf("%s", out)
		}
	}
	b := payBoard{lands: []string{"Cascading Cataracts", "Plains"}, hand: []string{"Ainok Bond-Kin", "Crystal Golem"}, tapped: map[string]bool{}}
	cands := []string{
		`{"kind":"pass"}`,
		`{"kind":"cast_spell","source":` + ref0("hand", "Ainok Bond-Kin") + `,"method":"normal"}`,
	}
	if got := pick(t, a, chooseLine(2, b, cands)); got != 1 {
		t.Fatalf("picked candidate %d (%s), want the planned cast of Ainok Bond-Kin", got, cands[got])
	}
}
