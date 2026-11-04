//go:build manabrew

package manabrew

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// A reveal cost that must share a colour says so, and its first Min cards are a
// legal answer (Illuminated Folio over two colourless cards and two blue ones).
func TestCardPickStatesItsSetConstraintAndFrontsALegalSet(t *testing.T) {
	d := &decision.Decision{Seq: 9, Kind: decision.KChoose, Min: 2, Max: 2, SetPropMode: decision.SetPropShared, Options: []decision.Option{
		{Index: 0, Kind: "revealcost", Obj: 110},
		{Index: 1, Kind: "revealcost", Obj: 85},
		{Index: 2, Kind: "revealcost", Obj: 115, SetProps: []string{"U"}},
		{Index: 3, Kind: "revealcost", Obj: 86, SetProps: []string{"U"}},
	}}
	tr := New("t", 1, nil)
	msg, _, err := tr.promptChoose(d, &view.View{})
	if err != nil {
		t.Fatal(err)
	}
	in := msg.Input.Value.(mb.ChooseCardsInput)
	if !strings.Contains(in.Presentation.Description, "All chosen targets must share a property.") {
		t.Fatalf("description %q does not state the constraint", in.Presentation.Description)
	}
	answer := mb.ChooseCardsDecision{ChosenCardIDs: []string{in.Cards[0].ID, in.Cards[1].ID}}
	if out := tr.parseChooseCards(answer, &Pending{Prompt: msg, Decision: d}); out.Err != nil {
		t.Fatalf("the offered prefix %v was refused: %+v", answer.ChosenCardIDs, out.Err)
	}
}

// A sum bound lists what each card counts for: a Pilot token's power says 1 and
// its crew value says 3.
func TestCardPickListsValuesUnderASumBound(t *testing.T) {
	d := &decision.Decision{Seq: 9, Kind: decision.KChoose, Min: 1, Max: 2, MinSum: 3, Options: []decision.Option{
		{Index: 0, Kind: "tapcost", Obj: 39, Value: 1},
		{Index: 1, Kind: "tapcost", Obj: 135, Value: 3},
	}}
	msg, _, err := New("t", 1, nil).promptChoose(d, &view.View{})
	if err != nil {
		t.Fatal(err)
	}
	const want = "Choose 1 to 2. Total value must be at least 3. Card values: o39 1, o135 3."
	if got := msg.Input.Value.(mb.ChooseCardsInput).Presentation.Description; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}
