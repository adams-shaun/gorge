package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionAllGivesTheChosenColourToEveryMatch: the controller names one
// colour for the whole sweep (Gains$ Choice), each permanent ValidCards$ names
// gains protection from it, and a permanent it does not name is left alone.
func TestProtectionAllGivesTheChosenColourToEveryMatch(t *testing.T) {
	t.Parallel()
	const fixture = "Name:Mass Ward\nManaCost:0\nTypes:Sorcery\nOracle:x\n"
	const mine = "Name:Ward Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const second = "Name:Ward Cub\nManaCost:1 G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	const theirs = "Name:Rival Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _ := putCounterTable(t, 61,
		[]*cards.Card{card(t, fixture), card(t, mine), card(t, second)}, []*cards.Card{card(t, theirs)})
	bear := moveSeeded(t, e, 0, mine, state.ZBattlefield)
	cub := moveSeeded(t, e, 0, second, state.ZBattlefield)
	rival := moveSeeded(t, e, 1, theirs, state.ZBattlefield)
	source := moveSeeded(t, e, 0, fixture, state.ZStack)
	e.G.Obj(source).Ability = &cards.SA{Kind: "SP", API: "ProtectionAll", Params: map[string]string{
		"ValidCards": "Creature.YouCtrl", "Gains": "Choice", "Choices": "AnyColor",
	}}
	e.pending = nil
	e.resolveTop()
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KChoose || len(ask.Options) != 5 {
		t.Fatalf("ProtectionAll ask = %+v, want a five-colour choice", ask)
	}
	red := -1
	for _, o := range ask.Options {
		if o.Label == "red" {
			red = o.Index
		}
	}
	if red < 0 {
		t.Fatalf("the choice does not offer red: %+v", ask.Options)
	}
	submitChoices(t, e, red)
	for _, id := range []state.ObjID{bear, cub} {
		if !e.HasKeyword(id, "Protection from red") {
			t.Errorf("creature %d lacks protection from red", id)
		}
		if e.HasKeyword(id, "Protection from white") {
			t.Errorf("creature %d has protection from white: the choice was not honoured", id)
		}
	}
	if e.HasKeyword(rival, "Protection from red") {
		t.Errorf("the opponent's creature gained protection from red")
	}
}
