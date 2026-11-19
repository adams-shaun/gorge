//go:build manabrew

package manabrew

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
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

// Crew 3 over four 1/1 Myr (the deadlock ManaBrew's real seat hit, G2): the
// floor only rides the description, so min must be the fewest cards that can
// reach it -- three -- or a client reading min answers one Myr and Validate
// refuses it. min-many cards then form a legal answer.
func TestCrewCardPickMinReachesTheSum(t *testing.T) {
	myr := func(i, obj int) decision.Option {
		return decision.Option{Index: i, Kind: "tapcost", Obj: state.ObjID(obj), Value: 1}
	}
	d := &decision.Decision{Seq: 9, Kind: decision.KChoose, Min: 1, Max: 4, MinSum: 3,
		Options: []decision.Option{myr(0, 444), myr(1, 445), myr(2, 446), myr(3, 447)}}
	tr := New("t", 1, nil)
	msg, _, err := tr.promptChoose(d, &view.View{})
	if err != nil {
		t.Fatal(err)
	}
	in := msg.Input.Value.(mb.ChooseCardsInput)
	if in.Min != 3 || in.Max != 4 {
		t.Fatalf("crew 3 over 1/1s: min %d max %d, want 3 and 4", in.Min, in.Max)
	}
	ids := make([]string, 0, in.Min)
	for _, c := range in.Cards[:in.Min] {
		ids = append(ids, c.ID)
	}
	if out := tr.parseChooseCards(mb.ChooseCardsDecision{ChosenCardIDs: ids}, &Pending{Prompt: msg, Decision: d}); out.Err != nil {
		t.Fatalf("min-many cards %v were refused: %+v", ids, out.Err)
	}
	// One 3-power Pilot alone crews 3: min stays 1, the largest value first.
	d.Options = []decision.Option{myr(0, 39), {Index: 1, Kind: "tapcost", Obj: 135, Value: 3}}
	d.Max = 2
	msg, _, _ = tr.promptChoose(d, &view.View{})
	in = msg.Input.Value.(mb.ChooseCardsInput)
	if in.Min != 1 {
		t.Fatalf("a 3-value card reaches the floor alone: min %d, want 1", in.Min)
	}
	// ...and it comes first, so the first min cards -- what ManaBrew's
	// SimpleAi answers -- are the Pilot, not the 1-power card (run mbw1 picked
	// a 2-power creature for Crew 3 before the order was fixed).
	first := []string{in.Cards[0].ID}
	if out := tr.parseChooseCards(mb.ChooseCardsDecision{ChosenCardIDs: first}, &Pending{Prompt: msg, Decision: d}); out.Err != nil {
		t.Fatalf("the first card %v was refused: %+v", first, out.Err)
	}
}
