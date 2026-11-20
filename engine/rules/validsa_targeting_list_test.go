package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Danitha, Sword of Hope: "Whenever you cast an Equipment spell or a spell
// that targets a creature you control, draw a card." Its ValidSA$ is a comma
// list whose second alternative carries the spaced IsTargeting argument
// (Spell.Equipment,Spell.IsTargeting Valid Creature.YouCtrl). Each
// alternative alone is a cast the trigger sees (CR 603.2); the clause used to
// be split on spaces and fail every cast closed.
func TestValidSATargetingListFiresDanitha(t *testing.T) {
	for _, tc := range []struct{ name, scenario string }{
		{"equipment spell", `{"name":"danitha-equipment","cr":["603.2"],"why":"an Equipment spell",
 "setup":{"p0":{"battlefield":["Danitha, Sword of Hope"],"hand":["Bonesplitter"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Bonesplitter","mana":"C"}]}`},
		{"spell targeting your creature", `{"name":"danitha-targeting","cr":["603.2"],"why":"a spell that targets a creature you control",
 "setup":{"p0":{"battlefield":["Danitha, Sword of Hope"],"hand":["Giant Growth"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Giant Growth","mana":"G","targets":["p0:Danitha, Sword of Hope"]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !danithaFires(t, tc.scenario) {
				t.Fatal("Danitha's SpellCast trigger never reached the stack")
			}
		})
	}
	// The negative: a spell targeting the OPPONENT's creature is neither
	// alternative, so the trigger stays silent.
	neg := `{"name":"danitha-negative","cr":["603.2"],"why":"a spell that targets an opponent's creature",
 "setup":{"p0":{"battlefield":["Danitha, Sword of Hope"],"hand":["Giant Growth"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Giant Growth","mana":"G","targets":["p1:Grizzly Bears"]}]}`
	if danithaFires(t, neg) {
		t.Fatal("Danitha fired for a spell targeting an opponent's creature")
	}
}

func danithaFires(t *testing.T, scenario string) bool {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("precondition: scenario did not play cleanly: %v", res.Fails)
	}
	for _, s := range res.Snapshots {
		for _, e := range s.Stack {
			if e.Kind == "ability" && strings.Contains(strings.ToLower(e.Source), "danitha") {
				return true
			}
		}
	}
	return false
}

// CR 109.2 / 110.1: "a creature you control" is a permanent, so a creature
// CARD in a graveyard is not one. Forge reads `IsTargeting Valid <spec>`
// through AbilityUtils.getDefinedObjects, whose `Valid ` form draws its
// candidates from the battlefield only. Generous Revival targets a creature
// card in p0's graveyard: Danitha must stay silent (cross-engine farm,
// FRA-s102 xe:RAND:0:5:w8 step 17; Forge and XMage agree).
func TestValidSAIsTargetingValidIsBattlefieldOnly(t *testing.T) {
	sc := `{"name":"danitha-graveyard-target","cr":["109.2","110.1"],"why":"a spell targeting a creature card in a graveyard is not targeting a creature you control",
 "setup":{"p0":{"battlefield":["Danitha, Sword of Hope"],"hand":["Generous Revival"],"graveyard":["Grizzly Bears"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Generous Revival","mana":"CCW","targets":["p0:Grizzly Bears"]}]}`
	if danithaFires(t, sc) {
		t.Fatal("Danitha fired for a spell targeting a creature card in a graveyard")
	}
}
