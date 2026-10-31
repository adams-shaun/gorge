package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// A copy made "except it's an artifact and loses all other card types" keeps
// its artifact subtypes and loses its creature subtypes (CR 205.1a).
func TestCopyExceptionCardTypeLossKeepsCorrelatedSubtypes(t *testing.T) {
	card := &cards.Card{Faces: []*cards.Face{{Name: "Test Blades", Types: []string{"Legendary", "Artifact", "Creature", "Equipment", "Lizard"}}}}
	o := state.NewGame([]string{"A"}).AddObject(card, 0)
	x := copyExceptions(o, copyExceptSpec{removeCardTypes: true, addTypes: []string{"Artifact"}})
	if want := []string{"Legendary", "Equipment", "Artifact"}; !slices.Equal(x.Types, want) {
		t.Fatalf("types = %v, want %v", x.Types, want)
	}
}
