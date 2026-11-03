package chars

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// typeMatchBoard answers an effect by its source, or by the types-so-far for any
// other spec, and counts the questions.
type typeMatchBoard struct {
	allTypesTestBoard
	matches *int
}

func (b typeMatchBoard) Matches(ce *state.ContinuousEffect, id state.ObjID, types, _ []string, _ state.Zone, _ PTBind) bool {
	*b.matches++
	if ce.Affects == "Card.Self" {
		return ce.Source == id
	}
	return slices.Contains(types, ce.Affects)
}

// An ability that animates its source leaves an effect per activation. Their
// number must not multiply the walk: Opal Acrolith's two hundred activations took
// a fifth of a second per decision.
func TestTypesWalkAsksEachSelfEffectOnce(t *testing.T) {
	card := &cards.Card{Faces: []*cards.Face{{Name: "Opal Acrolith", Types: []string{"Enchantment"}}}}
	game := state.NewGame([]string{"A"})
	id := game.AddObject(card, 0).ID
	act := make([]state.ContinuousEffect, 100)
	for i := range act {
		act[i] = state.ContinuousEffect{Layer: state.LType, Affects: "Card.Self", Source: id, Timestamp: uint32(i),
			RemoveCardTypes: true, AddTypes: []string{"Enchantment"}}
	}
	matches := 0
	b := typeMatchBoard{allTypesTestBoard{game: game, active: act}, &matches}
	types, _ := TypesAndAllCreatureTypes(b, act, id, 0)
	if !slices.Equal(types, []string{"Enchantment"}) {
		t.Fatalf("types = %v", types)
	}
	if matches != len(act) {
		t.Fatalf("%d matches for %d effects", matches, len(act))
	}
}

// Another effect can still depend on a Card.Self one (CR 613.8a): the Goblin grant
// has the earlier timestamp and waits for the effect that makes its object a
// creature.
func TestTypesWalkAppliesWhatDependsOnASelfEffectAfterIt(t *testing.T) {
	card := &cards.Card{Faces: []*cards.Face{{Name: "Opal Acrolith", Types: []string{"Enchantment"}}}}
	game := state.NewGame([]string{"A"})
	id := game.AddObject(card, 0).ID
	act := []state.ContinuousEffect{
		{Layer: state.LType, Affects: "Creature", Timestamp: 1, AddTypes: []string{"Goblin"}},
		{Layer: state.LType, Affects: "Card.Self", Source: id, Timestamp: 2, AddTypes: []string{"Creature"}},
	}
	matches := 0
	b := typeMatchBoard{allTypesTestBoard{game: game, active: act}, &matches}
	types, _ := TypesAndAllCreatureTypes(b, act, id, 0)
	if !slices.Contains(types, "Goblin") {
		t.Fatalf("types = %v, want the grant to wait for the Creature type", types)
	}
}
