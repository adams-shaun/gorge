package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
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

// CR 109.2 / 110.1: "a creature" is a permanent, so a creature CARD in a
// graveyard is not one. Forge reads `IsTargeting Valid <spec>` through
// AbilityUtils.getDefinedObjects, whose `Valid ` form draws its candidates
// from the battlefield only (cross-engine farm, FRA-s102 xe:RAND:0:5:w8 step
// 17). Head of the Class's `ValidSpell$ Spell.IsTargeting Valid Creature`
// W/B discount is the carrier whose bare base the old filter matched in any
// zone: a spell announcing a creature card in a graveyard earns no discount,
// while the same spell announcing a creature on the battlefield does.
func TestValidSAIsTargetingValidIsBattlefieldOnly(t *testing.T) {
	spellSrc := "Name:Wounded Spell\nManaCost:W B\nTypes:Sorcery\n" +
		"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +1 | NumDef$ +1\nOracle:x\n"
	e := handEngine(t, card(t, spellSrc))
	head := e.G.AddObject(corpusAlternativeCard(t, "Head of the Class"), 0)
	head.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{head.ID})
	spell := e.G.Zone(state.ZHand, 0)[0]
	live := battlePerm(t, e, 0, "Name:A Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	dead := e.G.AddObject(card(t, "Name:A Dead Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	dead.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{dead.ID})
	if e.G.Obj(dead.ID).Zone != state.ZGraveyard || !e.G.Obj(dead.ID).Face().IsCreature() ||
		e.G.Obj(live).Zone != state.ZBattlefield || e.G.Active != 0 {
		t.Fatal("precondition: a creature card in the graveyard, a creature on the battlefield, seat 0 active")
	}
	price := func(tg state.ObjID) int32 {
		m := e.costModifiersForTargets(0, spell, spellScope(""), []state.Target{{Obj: tg}})
		return m.Apply(e.parseCost("W B")).CMC()
	}
	if got := price(live); got != 0 {
		t.Fatalf("precondition: a battlefield creature target priced %d, want the discount to apply (0)", got)
	}
	if got := price(dead.ID); got != 2 {
		t.Fatalf("a creature card in a graveyard earned Head of the Class's discount: priced %d, want full 2", got)
	}
}
