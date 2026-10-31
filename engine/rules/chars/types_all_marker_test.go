package chars

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

type allTypesTestBoard struct {
	game   *state.Game
	active []state.ContinuousEffect
}

func (b allTypesTestBoard) Game() *state.Game                { return b.game }
func (b allTypesTestBoard) Active() []state.ContinuousEffect { return b.active }
func (allTypesTestBoard) Matches(*state.ContinuousEffect, state.ObjID, []string, []string, state.Zone, PTBind) bool {
	return true
}
func (allTypesTestBoard) StaticAmount(*state.ContinuousEffect, string, state.ObjID) int32 { return 0 }
func (allTypesTestBoard) ControllerOf(state.ObjID) state.PlayerID                         { return 0 }
func (allTypesTestBoard) LandTypeWords() []string                                         { return nil }
func (allTypesTestBoard) CDAContext(*state.Object, *cards.Face) *effects.Ctx              { return nil }
func (allTypesTestBoard) Host() effects.Host                                              { return nil }

func TestTypesAllCreatureMarkerClearedByLaterSubtypeRemoval(t *testing.T) {
	card := &cards.Card{Faces: []*cards.Face{{Name: "Test Bear", Types: []string{"Creature", "Bear"}}}}
	game := state.NewGame([]string{"A"})
	obj := game.AddObject(card, 0)
	id := obj.ID
	board := allTypesTestBoard{
		game: game,
		active: []state.ContinuousEffect{
			{Layer: state.LType, AddAllCreatureTypes: true},
			{Layer: state.LType, RemoveTypes: []string{"bear"}},
		},
	}

	if !containsType(obj.Face().Types, "Bear") {
		t.Fatal("precondition: test permanent does not start with Bear subtype")
	}
	types, all := TypesAndAllCreatureTypes(board, board.active, id, 0)
	if containsType(types, "Bear") {
		// The later effect must actually remove the subtype; this guards
		// against a vacuous effect setup, including case-insensitive matching.
		t.Fatalf("precondition: later type effect did not remove Bear: %v", types)
	}
	if all {
		t.Fatalf("later subtype removal left all-creature-types marker set: %v", types)
	}
}

func TestCardTypeRemovalAllCreatureMarker(t *testing.T) {
	for _, tc := range []struct {
		name  string
		added []string
		want  bool
	}{
		{"creature types go with Creature", []string{"Land"}, false},
		{"creature types stay with Creature", []string{"Artifact", "Creature"}, true},
		{"an added creature subtype replaces the set", []string{"Creature", "Insect"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card := &cards.Card{Faces: []*cards.Face{{Name: "Test Bear", Types: []string{"Creature", "Bear"}}}}
			game := state.NewGame([]string{"A"})
			id := game.AddObject(card, 0).ID
			board := allTypesTestBoard{
				game: game,
				active: []state.ContinuousEffect{
					{Layer: state.LType, AddAllCreatureTypes: true},
					{Layer: state.LType, RemoveCardTypes: true, AddTypes: tc.added},
				},
			}
			types, all := TypesAndAllCreatureTypes(board, board.active, id, 0)
			if all != tc.want {
				t.Fatalf("all-creature-types marker = %v, want %v (types %v)", all, tc.want, types)
			}
			if containsType(types, "Bear") != tc.want {
				t.Fatalf("Bear kept = %v, want %v (types %v)", !tc.want, tc.want, types)
			}
		})
	}
}

func containsType(types []string, want string) bool {
	for _, typ := range types {
		if strings.EqualFold(typ, want) {
			return true
		}
	}
	return false
}
