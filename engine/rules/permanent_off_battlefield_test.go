package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Forge's Permanent is a card-type test (Card.isPermanent, read), so it reads the
// same off the battlefield: a member a ConditionPresent$ or a Targeted$Valid
// count looks back at keeps its answer once the effect has moved it. Forge and
// XMage draw from Break the Spell on your own enchantment and exile a permanent
// spell Thranduil's Decree counters (played).
func TestPermanentBaseReadsTheCardTypeOffTheBattlefield(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	last := func(t *testing.T, scenario string) OracleSnapshot {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil || len(res.Fails) != 0 {
			t.Fatalf("err=%v fails=%v", err, res.Fails)
		}
		return res.Snapshots[len(res.Snapshots)-1]
	}
	breakSpell := func(owner string) string {
		seat := func(n string) string {
			if n == owner {
				return `,"battlefield":["Unstoppable Plan"]`
			}
			return ""
		}
		return `{"name":"break-the-spell","why":"a destroyed enchantment was a permanent you controlled",
"setup":{"p0":{"hand":["Break the Spell"],"library_top":["Island"]` + seat("p0") + `},"p1":{"library_top":["Island"]` + seat("p1") + `}},
"steps":[{"op":"cast","seat":0,"card":"p0:Break the Spell","mana":"W","targets":["` + owner + `:Unstoppable Plan"]},{"op":"resolve"}]}`
	}
	if got := len(last(t, breakSpell("p0")).Players[0].Hand); got != 1 {
		t.Fatalf("Break the Spell on your own enchantment: hand %d, want the 1 card drawn", got)
	}
	if got := len(last(t, breakSpell("p1")).Players[0].Hand); got != 0 {
		t.Fatalf("Break the Spell on an opponent's enchantment: hand %d, want 0", got)
	}
	decree := func(spell, cost string) string {
		return `{"name":"thranduils-decree","why":"a countered permanent spell is exiled",
"setup":{"p0":{"hand":["` + spell + `"]},"p1":{"hand":["Thranduil's Decree"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:` + spell + `","mana":"` + cost + `"},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Thranduil's Decree","mana":"CCCCUU","targets":["p0:` + spell + `"]},{"op":"resolve"}]}`
	}
	if s := last(t, decree("Grizzly Bears", "CG")); len(s.Players[0].Exile) != 1 || len(s.Players[0].Graveyard) != 0 {
		t.Fatalf("a countered creature spell: exile %v graveyard %v", s.Players[0].Exile, s.Players[0].Graveyard)
	}
	if s := last(t, decree("Opt", "U")); len(s.Players[0].Exile) != 0 || len(s.Players[0].Graveyard) != 1 {
		t.Fatalf("a countered instant: exile %v graveyard %v", s.Players[0].Exile, s.Players[0].Graveyard)
	}
}
