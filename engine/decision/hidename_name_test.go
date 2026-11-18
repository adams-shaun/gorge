package decision

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 201.4: a card-name choice ranges over the Oracle card reference, so a
// "name" option's label is the offered card name itself, not a reference to a
// game object. A face-down permanent whose face name is one of the offered
// names must not rewrite it: that would drop one legal answer from the ask
// and record a name ("a face-down permanent") no spell can carry, while
// telling the seat asked where in the sorted list the hidden name sits. An
// object option (Obj set) still reads the shown text.
func TestHideNameKeepsCardNameChoiceOptions(t *testing.T) {
	const name = "Keeper of Secrets"
	d := &Decision{Prompt: "Choose a card name", Options: []Option{
		{Index: 0, Kind: "name", Label: name},
		{Index: 1, Kind: "name", Label: "Cheering Fanatic"},
		{Index: 2, Kind: "target", Label: name, Obj: 9},
	}}
	// Precondition: the hidden object's name IS one of the offered names, so
	// the rewrite would have hit the name options had it not been excluded,
	// and the object option is hidden by the same call.
	d.HideName(7, name, "a face-down permanent", func(id state.ObjID) bool { return false })
	if got := d.Options[0].Label; got != name {
		t.Fatalf("name option label = %q, want %q (precondition: the offered name equals the hidden face name)", got, name)
	}
	if got := d.Options[1].Label; got != "Cheering Fanatic" {
		t.Fatalf("unrelated name option label = %q, want Cheering Fanatic", got)
	}
	if got := d.Options[2].Label; got != "a face-down permanent" {
		t.Fatalf("object option label = %q, want it hidden (precondition: HideName still rewrites object labels)", got)
	}
}
