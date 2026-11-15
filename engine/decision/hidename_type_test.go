package decision

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A face-down Forest hides the name "Forest" from an opponent's labels, but a
// basic land type choice offers the word Forest, which is not the card's name.
func TestHideNameLeavesTypeWordOptions(t *testing.T) {
	d := &Decision{Prompt: "Choose a land type", Options: []Option{
		{Index: 0, Kind: "type", Label: "Forest"},
		{Index: 1, Kind: "target", Label: "Forest"},
	}}
	d.HideName(7, "Forest", "a face-down permanent", func(id state.ObjID) bool { return false })
	if got := d.Options[0].Label; got != "Forest" {
		t.Fatalf("type option label = %q, want Forest", got)
	}
	if got := d.Options[1].Label; got != "a face-down permanent" {
		t.Fatalf("object option label = %q, want it hidden (precondition: HideName still rewrites object labels)", got)
	}
}
